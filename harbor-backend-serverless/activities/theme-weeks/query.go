package main

var query = `
with subscriptions as(
	select event_id
	from events_subscriptions
	where user_id = %s
), points as (
    select
       sum(c.readiness_points) as total,
       sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
       c.activity_id
	from chapters c
	inner join event_chapters ec on ec.chapter_id = c.id
    inner join subscriptions s on s.event_id = ec.event_id
    left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (?)
    group by c.activity_id
), theme_week as (
	select
		case when extract(isodow from created_at) in (5,6,7)
		then created_at + cast(format('%%s days', (7 - extract(isodow from created_at)) + 7) as interval)
		else created_at + cast(format('%%s days', 7 - extract(isodow from created_at)) as interval) end as last_day
	from users
	where id = %s
)
select
	ats.id as theme_id,
	ats.theme,
	ats.ordering,
	a.id as activity_id,
	a.activity_level_id as level_id,
	a.name,
	case
    	when points.total = 0 then 0
        else points.current::real/points.total::real end as readiness,
    extract(days from (now() - (select last_day from theme_week))) as days_elapsed
from points
inner join activities a on a.id = points.activity_id
inner join activity_themes ats on a.theme_id = ats.id
order by ordering, theme, level_id`
