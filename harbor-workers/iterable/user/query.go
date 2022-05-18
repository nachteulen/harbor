package main

const query = `
select
	u.id,
	coalesce(u.first_name, '') as first_name,
	coalesce(u.last_name, '') as last_name,
	coalesce(email, social_email, '') as email,
	a.zipcode,
	zl.city,
	zl.state,
	(select json_agg(json_build_object('name', e.name, 'level', level_id))
	 from risk_profiles rp, jsonb_to_recordset(profile) x(risk_id int, level_id int)
	 inner join events e on e.id = risk_id
	 where rp.id = a.risk_profile_id) as risk_profile,
	(select format('["%s"]', string_agg(e.name::text, '","'))
	 from events_subscriptions es
	 inner join events e on es.event_id = e.id
	 where user_id = u.id) as subscribed_risks
from users u
left join addresses a on a.id = u.address_id
left join zipcode_locations zl on zl.id = a.zipcode::integer
where u.id = $1`
