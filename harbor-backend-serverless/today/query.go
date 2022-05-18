package main

const query = `
with found_user as (
    select id
    from users
    where id = $1
), start_week as (
    select
        case when extract(isodow from created_at) in (5,6,7)
        then created_at + cast(format('%s days', (7 - extract(isodow from created_at)) + 7) as interval)
        else created_at + cast(format('%s days', 7 - extract(isodow from created_at)) as interval)
        end as last_day
    from users
    where id = (select id from found_user)
), days_elapsed as (
    select extract(days from (now() - (select last_day from start_week))) as days_elapsed
    from start_week
), user_schedule as (
    select schedule
    from weekly_schedules
    where user_id = (select id from found_user)
), schedule as (
    select coalesce((select schedule from user_schedule), (
        select array_to_json(array_agg(jsonb_build_object('id', id, 'type', 'theme')))::jsonb
        from (
            select id
            from activity_themes
            order by ordering
        ) o1)
    ) as schedule
), is_first_cycle as (
    select
        (select days_elapsed from days_elapsed)
        <= (select (jsonb_array_length(schedule) - 1) * 7 from schedule) as is_first_cycle
), week_idx as (
    select case
        when (select days_elapsed from days_elapsed) < 1 then 0
        else (
            select ceil( (select days_elapsed from days_elapsed) / 7::real )
        )::int % (select jsonb_array_length(schedule) from schedule) end as week_idx
), ownerships as (
    select oid::int
    from (select jsonb_array_elements($2) as oid) oids
), addr as (
    select profile
    from users u
    join addresses a on u.address_id = a.id
    join risk_profiles rp on a.risk_profile_id = rp.id
    where u.id = (select id from found_user)
), risk_profile as (
    select risk_id, level_id
    from addr, jsonb_to_recordset(profile) x(risk_id int, level_id int)
), risk_subscriptions as (
    select id, event_id
    from events_subscriptions
    where ownership_id in (select oid from ownerships)
), shared_completed_chapters as (
    select id, chapter_id
    from completed_chapters
    where ownership_id in (select oid from ownerships)
), all_points as (
    select
        sum(c.readiness_points) as total,
        sum(case when cc.id is not null then c.readiness_points else 0 end) as current
    from chapters c
    left join shared_completed_chapters cc on cc.chapter_id = c.id
), chapters_readiness as (
    select case
        when (select total from all_points) = 0 then 0
        else sum((select current from all_points) / (select total from all_points)::decimal) end as readiness
), risk_plans as (
    select
        id,
        plan_id,
        name,
        'risk' as type,
        related_theme_ids
    from events
), activity_ids as (
    select id
    from activities
    where theme_id in (select id from activity_themes)
), activity_points as (
    select
       sum(c.readiness_points) as total,
       sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
       c.activity_id as id,
       'activity' as type
    from chapters c
    inner join event_chapters ec on ec.chapter_id = c.id
    inner join risk_subscriptions s on s.event_id = ec.event_id
    left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (select oid from ownerships)
    where c.activity_id in (select id from activity_ids)
    group by c.activity_id
), theme_points as (
    select
        a.theme_id as id,
        'theme' as type,
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
    select plan_id from risk_plans
    union
    select plan_id from theme_plans
), plans_data as (
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
        where version_key <= $3 and jsonb_typeof(form_ids) = 'object'
    ) o1 on true
    where id in (select distinct plan_id from plan_ids)
), plan_point_data as (
    select p.id as plan_id, p.name, p.max_points, coalesce(fia.points, 0) as points
    from plans_data p
    join forms f on f.id in (
        select fid::int from (select jsonb_array_elements(p.form_ids) as fid) fids
    )
    join form_inputs fi on fi.id in (
        select in_id::int from (select jsonb_array_elements(input_ids) as in_id) in_ids
    )
    left join form_input_answers fia on case
        when fi.is_global = true then fia.input_id = fi.id
        else (fia.plan_id = p.id and fia.form_id = f.id and fia.input_id = fi.id) end
        and fia.household_id = $4
), plan_point_sums as (
    select plan_id, points, o2.max_points
    from (
        select plan_id, sum(points) as points
        from plan_point_data
        group by plan_id
    ) o1
    left join lateral (
        select max_points
        from plan_point_data
        where plan_id = o1.plan_id
        limit 1
    ) o2 on true
), plans_progress as (
    select
        plan_id,
        case
            when max_points = 0 then 0
            else (points / max_points::real) end as progress
    from plan_point_sums
), plans_readiness as (
    select case
        when sum_max = 0 then 0
        else sum_points / sum_max::real end as readiness
    from (
        select sum(points) as sum_points, sum(max_points) as sum_max
        from plan_point_sums
    ) o1
), total_readiness as (
    select
        (select readiness * 0.85 from plans_readiness)
        + (select readiness * 0.15 from chapters_readiness) as readiness
), rank as (
    select case
        when (select readiness from total_readiness) < 0.03 then 'New to this'
        when (select readiness from total_readiness) >= 0.03 and (select readiness from total_readiness) < 0.05 then 'Getting going'
        when (select readiness from total_readiness) >= 0.05 and (select readiness from total_readiness) < 0.11 then 'Feeling calm'
        when (select readiness from total_readiness) >= 0.11 and (select readiness from total_readiness) < 0.22 then 'Hanging tough'
        when (select readiness from total_readiness) >= 0.22 and (select readiness from total_readiness) < 0.38 then 'We got this'
        when (select readiness from total_readiness) >= 0.38 and (select readiness from total_readiness) < 0.56 then 'Free and Fearless'
        when (select readiness from total_readiness) >= 0.56 and (select readiness from total_readiness) < 0.72 then 'Feeling bold'
        when (select readiness from total_readiness) >= 0.72 and (select readiness from total_readiness) < 0.84 then 'Not kidding around'
        when (select readiness from total_readiness) >= 0.84 and (select readiness from total_readiness) < 0.9 then 'Jack-of-all-trades'
        else 'Readiness Expert' end as rank
), risks_json as (
    select json_object_agg(i.id, i.risk_data)
    from (
        select
            rp.id,
            jsonb_build_object(
                'name', name,
                'progress', pp.progress,
                'relatedThemeIDs', rp.related_theme_ids,
                'level', r_prof.level_id,
                'levelText', rl.attrs ->> 'text',
                'levelColor', rl.attrs ->> 'color'
            ) as risk_data
        from risk_plans rp
        join plans_progress pp on pp.plan_id = rp.plan_id
        left join risk_profile r_prof on r_prof.risk_id = id
        left join risk_levels rl on rl.level = r_prof.level_id
    ) i
), scheduled_risks_order as (
    select id as risk_id, row_number() over()
    from (
        select id, type
        from
            weekly_schedules,
            jsonb_to_recordset(schedule) x(id int, type text)
        where user_id = (select id from found_user)
    ) x1
    where type = 'risk'
), ordered_risks as (
    select id, subscribed
    from (
        select event_id as id, true as subscribed, level_id
        from (
            select event_id, level_id
            from risk_subscriptions rs
            left join risk_profile rp on rp.risk_id = rs.event_id
        ) a1
        union
        select id, false as subscribed, level_id
        from (
            select id, level_id
            from events e
            left join risk_profile rp on rp.risk_id = e.id
            where id not in (select event_id from risk_subscriptions)
        ) a2
    ) a3
    left join scheduled_risks_order sro on sro.risk_id = a3.id
    order by sro.row_number, subscribed desc, level_id desc, id
), risks_order_json as (
    select array_to_json(array_agg(json_build_object(
        'id', id,
        'subscribed', subscribed
    ))) as risks_ordering
    from ordered_risks
), themes_json as (
    select json_object_agg(i.id, i.theme_data)
    from (
        select
            id,
            jsonb_build_object(
                'id', id,
                'name', name,
                'progress', (tp.progress * 0.2) + (pp.progress * 0.8)
            ) as theme_data
        from theme_plans tp
        join plans_progress pp on pp.plan_id = tp.plan_id
    ) i
), found_null_schedule as (
    select (select schedule from user_schedule) is null
)
select
    (select week_idx from week_idx),
    (select schedule from schedule) as schedule_json,
    (select * from themes_json) as themes_json,
    (select * from risks_json) as risks_json,
    (select risks_ordering from risks_order_json),
    (select readiness from total_readiness),
    (select rank from rank),
    (select * from found_null_schedule) as found_null_schedule,
    (select is_first_cycle from is_first_cycle)`
