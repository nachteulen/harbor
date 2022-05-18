package main

const query = `
with found_user as (
    select id, address_id
    from users
    where id = $1
), addr as (
    select profile
    from addresses a
    join risk_profiles rp on a.risk_profile_id = rp.id
    where a.id = (select address_id from found_user)
), risk_profile as (
    select risk_id, level_id
    from addr, jsonb_to_recordset(profile) x(risk_id int, level_id int)
), ownerships as (
    select oid::int
    from (select jsonb_array_elements($2) as oid) oids
), subscriptions as (
    select id, event_id
    from events_subscriptions
    where user_id = (select id from found_user)
), shared_completed_chapters as (
    select id, chapter_id
    from completed_chapters
    where ownership_id in (select oid from ownerships)
), risk as (
    select
        e.id,
        e.plan_id,
        e.related_theme_ids,
        e.name,
        e.disclaimer,
        e.description,
        is_priority,
        case when s.id is not null then true else false end as is_subscribed,
        level_id,
        rl.attrs ->> 'text' as level_text,
        rl.attrs ->> 'color' as level_color
    from
        events e
    left join subscriptions s on s.event_id = e.id
    left join risk_profile rp on rp.risk_id = e.id
    left join risk_levels rl on rl.level = rp.level_id
    where e.id = $3
), themes as (
    select plan_id
    from activity_themes where id in (
        select tid::int
        from (select jsonb_array_elements((select related_theme_ids from risk)) as tid) i
    )
), plans as (
    select
        id,
        name,
        max_points,
        case
            when o1.max_version_key is null then form_ids
            else form_ids->o1.max_version_key end as form_ids
    from plans p
    left join lateral (
        select max(version_key)::text as max_version_key
        from (select jsonb_object_keys(form_ids)::int as version_key) o2
        where version_key <= $4 and jsonb_typeof(form_ids) = 'object'
    ) o1 on true
    where id in (
        select plan_id from risk
        union select plan_id from themes
    )
), activity_ids as (
    select id
    from activities where theme_id in (
        select tid::int
        from (select jsonb_array_elements((select related_theme_ids from risk)) as tid) i
    )
), activity_points as (
    select
       sum(c.readiness_points) as total,
       sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
       c.activity_id as id
    from chapters c
    inner join event_chapters ec on ec.chapter_id = c.id
    inner join subscriptions s on s.event_id = ec.event_id
    left join shared_completed_chapters cc on cc.chapter_id = c.id
    where c.activity_id in (select id from activity_ids)
    group by c.activity_id
), theme_points as (
    select
        a.theme_id as id,

        sum(total) as total,
        sum(current) as current
    from activity_points ap
    join activities a on a.id = ap.id
    group by a.theme_id
), theme_plans as (
    select
        tp.id,
        ats.plan_id,
        ats.theme as name,
        'theme' as type,
        case when tp.total = 0 then 0 else (tp.current / tp.total::real) end as progress
    from theme_points tp
    join activity_themes ats on ats.id = tp.id
), plan_ids as (
    select plan_id from risk
    union
    select plan_id from theme_plans
), plan_point_data as (
    select p.id as plan_id, p.name, p.max_points, coalesce(fia.points, 0) as points
    from plans p
    join forms f on f.id in (
        select fid::int from (select jsonb_array_elements(form_ids) as fid) fids
    )
    join form_inputs fi on fi.id in (
        select in_id::int from (select jsonb_array_elements(input_ids) as in_id) in_ids
    )
    left join form_input_answers fia on case
        when fi.is_global = true then fia.input_id = fi.id
        else (fia.plan_id = p.id and fia.form_id = f.id and fia.input_id = fi.id) end
        and fia.household_id = $5
    where p.id in (select distinct plan_id from plan_ids)
), plan_point_sums as (
    select plan_id, sum(points)
    from plan_point_data
    group by plan_id
), plan_progress as (
    select distinct
        ppd.plan_id,
        case when ppd.max_points = 0 then 0 else (pps.sum / ppd.max_points::real) end as progress
    from plan_point_data ppd
    join plan_point_sums pps on pps.plan_id = ppd.plan_id
), plan_forms as (
 	select f.id, row_number() over(), f.text, f.input_ids
	from (
		select jsonb_array_elements(form_ids) as fid
		from plans
		where id = (select plan_id from risk)
	) fids
	join forms f on f.id = fid::int
), forms_json as (
	select array_to_json(array_agg(row_to_json(i)))
	from (
		select id, text, input_ids as "inputIDs"
		from plan_forms
		order by row_number asc
	) i
), input_ids as (
	select distinct(jsonb_array_elements(input_ids)) as in_id
 	from plan_forms
), form_to_input as (
	select pf.id::int as form_id, input_id::int
 	from plan_forms pf
	left join lateral jsonb_array_elements(input_ids) input_id on true
), inputs as (
	select id, name, type, meta, data_type, is_global
	from form_inputs
	where id in (select in_id::int from input_ids)
), input_answers as (
	select fia.form_id, fi.id as input_id, fia.id as answer_id, fia.value, fia.points
	from form_to_input fti
	join inputs fi on fi.id = fti.input_id
	left join form_input_answers fia on case
		when fi.is_global = true then fia.input_id = fi.id
		else (
			fia.plan_id = (select plan_id from risk)
			and fia.form_id = fti.form_id
			and fia.input_id = fi.id
		) end
	where fia.household_id = $6
), input_answers_json as (
 	select json_object_agg(
 		format('%s:%s', i.form_id, i.input_id),
 		i.answer_data
 	)
    from (
        select
         	ia.form_id,
          	ia.input_id,
            jsonb_build_object(
                'answerID', ia.answer_id,
                'answerValue', ia.value
            ) as answer_data
        from input_answers ia
    ) i
), inputs_json as (
	select json_object_agg(
		id,
		json_build_object(
			'type', type,
			'dataType', data_type,
			'meta', meta,
			'isGlobal', is_global
		)
	)
	from (
		select i.id, i.type, i.data_type, i.meta, i.is_global
		from inputs i
	) i
), ordered_guide as (
    select
        event_id,
        e.name as event_name,
        format('"%s"', em.name)::jsonb as mode_name,
        jsonb_set(
            raw_value::jsonb,
            '{name}'::text[],
            format('"%s"', em.name)::jsonb
        ) || jsonb_set(
            raw_value::jsonb,
            '{emergencyModeTypeID}'::text[],
            format('%s', emergency_mode_type_id)::jsonb
        ) as raw_value,
        emergency_mode_type_id
    from emergency_modes em
    join events e on e.id = em.event_id
    where event_id = (select id from risk)
    order by emergency_mode_type_id
), guide_json as (
    select json_build_object(
        'eventID', event_id,
        'eventName', event_name,
        'steps', steps)
    from (
        select
            event_id,
            event_name,
            json_agg(json_build_object('rawValue', raw_value)) as steps
        from ordered_guide
        group by event_id, event_name
        order by event_id
    ) og
), themes_json as (
    select array_to_json(array_agg(jsonb_build_object(
        'id', id,
        'name', name,
        'progress', (tp.progress * 0.2) + (pp.progress * 0.8)
    )))
    from theme_plans tp
    join plan_progress pp on pp.plan_id = tp.plan_id
)
select
	r.id,
    r.name,
    r.disclaimer,
    r.description,
    r.is_priority,
    r.is_subscribed,
    r.level_id,
    r.level_text,
    r.level_color,
    p.id as plan_id,
    p.name as plan_name,
	(select * from forms_json) as forms_json,
	(select * from inputs_json) as inputs_json,
	(select * from input_answers_json) as input_answers_json,
	(select * from guide_json) as guide_json,
	(select progress from plan_progress where plan_id = r.plan_id) as risk_plan_progress,
	(select * from themes_json) as themes_json
from risk r
join plans p on p.id = r.plan_id`
