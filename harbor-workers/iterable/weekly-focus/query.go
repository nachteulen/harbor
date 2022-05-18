package main

const selectQuery = `
with user_data as (
	select
		u.id,
		email,
		social_email,
		social_email_verified
	from users u
	where id > $1 and id != 1
	order by 1 limit 50
)
select id, coalesce(email, social_email) as email
from user_data
where
	email is not null
	or (social_email is not null and social_email_verified = true)`
