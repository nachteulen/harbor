package main

const query = `
select
	coalesce(u.email, u.social_email) as email,
	concat(zl.city, ', ', zl.state_abbr)  as address,
	a.latitude,
	a.longitude
from users u
left join addresses a on address_id = a.id
left join zipcode_locations zl on a.zipcode::int = zl.id
where
	(u.email is not null or u.social_email is not null)
	and a.latitude >= $1 and a.latitude <= $2
	and a.longitude >= $3 and a.longitude <= $4`