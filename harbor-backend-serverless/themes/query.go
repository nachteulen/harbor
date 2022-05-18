package main

const query = `
with theme as (
	select
		theme,
		description,
		plan_id,
		related_risk_ids,
		inventory_category_ids
	from activity_themes
	where id = $1
), plan as (
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
        where version_key <= $2 and jsonb_typeof(form_ids) = 'object'
    ) o1 on true
    where id = (select plan_id from theme)
), plan_forms as (
	select f.id, row_number() over(), f.text, f.input_ids, f.snack_ids
	from (select jsonb_array_elements(form_ids) as fid from plan) fids
	join forms f on f.id = fid::int
), forms_json as (
	select array_to_json(array_agg(row_to_json(i)))
	from (
		select id, text, input_ids as "inputIDs", snack_ids as "snackIDs"
		from plan_forms
		order by row_number asc
	) i
), input_ids as (
	select distinct(jsonb_array_elements(input_ids)) as in_id
	from plan_forms
), snack_ids as (
	select distinct(jsonb_array_elements(snack_ids)) as s_id
	from plan_forms
), snacks as (
	select id, snack_type, title, body, meta
	from snacks
	where id in (select s_id::int from snack_ids)
), form_to_input as (
	select pf.id::int as form_id, input_id::int
	from plan_forms pf
	left join lateral jsonb_array_elements(input_ids) input_id on true
), inputs as (
	select id, type, meta, data_type, is_global
	from form_inputs
	where id in (select in_id::int from input_ids)
), input_answers as (
	select
		fia.form_id,
		fi.id as input_id,
		fia.id as answer_id,
		fia.value,
		fia.points,
		fia.meta
	from form_to_input fti
	join inputs fi on fi.id = fti.input_id
	left join form_input_answers fia on case
		when fi.is_global = true then fia.input_id = fi.id
		else (
			fia.plan_id = (select id from plan)
			and fia.form_id = fti.form_id
			and fia.input_id = fi.id
		) end
	where fia.household_id = $3
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
                'answerValue', ia.value,
                'answerMeta', ia.meta
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
), snacks_json as (
	select json_object_agg(
		id,
		json_build_object(
			'type', snack_type,
			'title', title,
			'body', body,
			'meta', meta
		)
	)
	from (
		select s.id, s.snack_type, s.title, s.body, s.meta
		from snacks s
	) o1
), subscriptions as(
	select event_id
	from events_subscriptions
	where user_id = $4
), points as (
   select
    	sum(c.readiness_points) as total,
      	sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
      	c.activity_id
	from chapters c
	inner join event_chapters ec on ec.chapter_id = c.id
    inner join subscriptions s on s.event_id = ec.event_id
    left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (
		select oid::int
		from (select jsonb_array_elements($5) as oid) oids
	)
    where activity_id in (select id from activities where theme_id = $6)
    group by c.activity_id
), activities as (
	select
		a.id as activity_id,
		a.activity_level_id as level_id,
		a.name,
		case
	    	when points.total = 0 then 0
	      else points.current::real/points.total::real end as readiness
	from points
	inner join activities a on a.id = points.activity_id
	order by level_id
), total_activity_readiness as (
	select case
		when all_total = 0 then 0
		else all_current / all_total end as points
	from (
		select sum(current) all_current, sum(total) all_total
		from points
	) o1
), activities_json as (
	select json_agg(json_build_object(
		'activityID', activity_id,
		'levelID', level_id,
		'name', name,
		'readiness', readiness
	))
	from activities
), readiness as (
	select (
		(select coalesce((select sum(points) from input_answers), 0)
		/ (select max_points from plan)::real) * 0.8)
		+ ((select points from total_activity_readiness) * 0.2) as points
), inventory_ids as (
	select jsonb_array_elements((select inventory_category_ids from theme)) as inv_id
), inventories_json as (
	select array_to_json(array_agg(row_to_json(r))) from (
		select id, name
		from inventory_categories
		where id in (select inv_id::int from inventory_ids)
	) r
)
select
	t.theme,
	t.description,
	t.related_risk_ids,
	p.id,
	p.name,
	(select * from inputs_json) as inputs_json,
	(select * from snacks_json) as snacks_json,
	(select * from forms_json) as forms_json,
	(select * from activities_json) as activities_json,
	(select * from inventories_json) as inventories_json,
	(select points from readiness) as readiness,
	(select * from input_answers_json) as input_answers_json
from theme t
join plan p on p.id = t.plan_id`
