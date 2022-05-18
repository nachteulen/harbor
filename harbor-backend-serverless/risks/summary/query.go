package main

var query = `
with subscribed as(
	select event_id
	from events_subscriptions
	where ownership_id in (?)
), readiness as (
	select
    	sum(c.readiness_points) as total,
	    sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
    	ec.event_id
	from chapters c
    	left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (?)
    	inner join event_chapters ec on ec.chapter_id = c.id
    	inner join subscribed s on s.event_id = ec.event_id
	group by
    	ec.event_id
) select
	case when (select count(*) from subscribed) = 0 then 0
	else sum(current::decimal / total) / (select count(*) from subscribed) end as avg,
	(select count(*) from subscribed) as count
from readiness`
