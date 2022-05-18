package main

const query = `
with user_list as (
	select distinct(l.user_id)
	from users u
	left join subscriptions s on s.user_id = u.id
	left join partner_organizations po on po.id = u.partner_id and po.is_premium = true
	join lateral (
		select hu2.user_id
		from household_users hu1
		inner join household_users hu2 on hu2.household_id = hu1.household_id
		where hu1.user_id = u.id
	) as l on true
	where
		l.user_id > $1
		and l.user_id is not null
		and (s.id is not null or po.id is not null)
	order by 1 limit 100
), user_data as (
	select
		u.id,
		email,
		social_email,
		social_email_verified,
		receipt_raw->'latest_receipt_info'->0->>'is_trial_period' as is_trial,
		s.is_active,
		s.name,
		case
			when po.id is not null and u.created_at + make_interval(po.days_premium) > now() then true
			when po.id is not null and u.created_at + make_interval(po.days_premium) < now() then false
			else null
		end as is_active_corporate_premium
	from user_list ul
	inner join users u on u.id = ul.user_id
	left join partner_organizations po on po.id = u.partner_id and po.is_premium = true
	left join subscriptions s on s.user_id = u.id
	left join apple_subscriptions asub on asub.id = s.apple_subscription_id
)
select
	id,
	coalesce(email, social_email) as email,
	is_trial::boolean,
	is_active,
	split_part(name, '.', 3) as name,
	is_active_corporate_premium
from user_data
where
	email is not null
	or (social_email is not null and social_email_verified = true)`
