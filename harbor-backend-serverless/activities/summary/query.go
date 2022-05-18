package main

var query = `
with subscriptions as(
	select event_id
	from events_subscriptions
	where ownership_id in (?)
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
)
select
	a.id,
	a.activity_level_id,
	a.activity_group_id,
	ats.theme,
	icon_image.path as icon_image_path,
	case
    	when points.total = 0 then 0
        else points.current::decimal/points.total end as readiness
from points
inner join activities a on a.id = points.activity_id
inner join activity_themes ats on a.theme_id = ats.id
left join files icon_image on icon_image.uuid = a.icon_uuid
order by ats.ordering, activity_level_id`
