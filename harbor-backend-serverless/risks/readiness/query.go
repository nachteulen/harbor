package main

var query = `
with points as (
	select
		sum(c.readiness_points) as total,
		sum(case when cc.id is not null then c.readiness_points else 0 end) as current
	from chapters c
	left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in (?)
) select
	case when (select total from points) = 0 then 0
	else sum((select current from points)::decimal / (select total from points)) end as readiness
from points`
