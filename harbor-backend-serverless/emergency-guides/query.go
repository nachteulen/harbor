package main

const query = `
with guides_json as (
	select json_build_object(
		'id', id,
		'name', name,
		'emergencyCodes', emergency_codes
	) as o1
	from events e
	where e.id in ((select distinct(event_id) from emergency_modes))
	order by id
)
select json_build_object('guides', array_to_json(array_agg(o1)))
from guides_json`
