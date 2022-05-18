package main

const selectQuery = "select exists(select 1 from risk_profiles where id = $1)"

// "do" the update to force the id to return
const insertProfileQuery = `
with inserted as (
	insert into risk_profiles (id, profile, local_authorities)
	values ($1, $2, $3)
	on conflict (id) do update set id = excluded.id returning id, profile
), user_data as (
	select id, address_id
	from users
	where id = $4
),
updated as (
	update addresses
	set risk_profile_id = (select id from inserted)
	where id = (select address_id from user_data)
),
high_risk as (
	select risk_id
	from jsonb_to_recordset((select profile from inserted))
	as x(risk_id int, level_id int)
	where level_id > 2
),
ownership as (
	select o.id
	from household_users hu
	inner join ownerships o on o.household_user_id = hu.id
	where hu.user_id = (select id from user_data) and o.ownership_type_id = 1
)
insert into events_subscriptions (
	event_id,
	ownership_id,
	user_id
)
select
	risk_id,
	(select id from ownership),
	(select id from user_data)
from high_risk
on conflict(user_id, event_id) do nothing`

const updateAddressQuery = `
with user_data as (
	select id, address_id
	from users
	where id = $1
),
updated as (
    update addresses
	set risk_profile_id = $2
	where id = (select address_id from user_data)
	returning risk_profile_id
),
high_risk as (
	select risk_id
	from risk_profiles, jsonb_to_recordset(profile)
	as x(risk_id int, level_id int)
	where id = (select risk_profile_id from updated)
	and level_id > 2
),
ownership as (
	select o.id
	from household_users hu
	inner join ownerships o on o.household_user_id = hu.id
	where hu.user_id = (select id from user_data)
	and o.ownership_type_id = 1
)
insert into events_subscriptions (
	event_id,
	ownership_id,
	user_id
)
select
	risk_id,
	(select id from ownership),
	(select id from user_data)
from high_risk
on conflict(user_id, event_id) do nothing`
