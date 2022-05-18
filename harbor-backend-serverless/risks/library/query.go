package main

var query = `
with addr as (
    select profile
    from users u
    join addresses a on u.address_id = a.id
    join risk_profiles rp on a.risk_profile_id = rp.id
    where u.id = %s
),
risk_profile as (
    select risk_id, level_id
    from addr, jsonb_to_recordset(profile) x(risk_id int, level_id int)
),
subscriptions as (
    select id, event_id
    from events_subscriptions
    where ownership_id in (?)
),
event_points_cte as (
    select
        sum(c.readiness_points) as total,
        sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
        ec.event_id
    from chapters c
    left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (?)
    inner join event_chapters ec on ec.chapter_id = c.id
    group by ec.event_id
),
events as (
    select
        e.id,
        e.name,
        e.disclaimer,
        is_priority,
        coalesce(case
            when event_points_cte.total = 0 then 0
            else event_points_cte.current::decimal/event_points_cte.total
            end, 0) as readiness,
        case when s.id is not null then true else false end as is_subscribed,
        level_id,
        rl.attrs ->> 'text' as level_text,
        rl.attrs ->> 'color' as level_color
    from
        events e
    left join subscriptions s on s.event_id = e.id
    left join event_points_cte on event_points_cte.event_id = e.id
    left join risk_profile rp on rp.risk_id = e.id
    left join risk_levels rl on rl.level = rp.level_id
    where e.enabled = true
)
select *
from events
order by case when is_subscribed then 1 end, level_id desc`
