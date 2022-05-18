package main

// "do" the update to force the id to return
const query = `
with inserted as (
	insert into risk_profiles (id, profile, local_authorities)
	values ($1, $2, $3)
	on conflict (id) do update set id = excluded.id returning id, profile
), updated as (
	update addresses
	set
		latitude = $4,
		longitude = $5,
		address = $6,
		zipcode = $7,
		risk_profile_id = (select id from inserted)
	where id = $8
), high_risk as (
	select risk_id, level_id
	from jsonb_to_recordset((select profile from inserted))
	as x(risk_id int, level_id int)
	where level_id > 2
), ownership_ids as (
    select oid::int
    from (select jsonb_array_elements($9) as oid) oids
), ownership as (
	select id
	from ownerships
	where id in (select oid from ownership_ids) and ownership_type_id = 1
), _ as (
	insert into events_subscriptions (
		event_id,
		ownership_id,
		user_id
	)
	select
		risk_id,
		(select id from ownership),
		($10)
	from high_risk
	on conflict(user_id, event_id) do nothing
)
select
	risk_id,
	level_id,
	rl.attrs ->> 'text' as risk_text,
    rl.attrs ->> 'color' as risk_color,
    e.name
from high_risk rp
left join events e on e.id = risk_id
left join risk_levels rl on rl.level = rp.level_id
where risk_id != 5
order by level_id desc`
