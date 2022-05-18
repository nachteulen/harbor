package main

const query = `
with found_user as (
	select id, address_id
	from users u
	where u.id = $1
), addr as (
    select profile
    from addresses a
    join risk_profiles rp on a.risk_profile_id = rp.id
    where a.id = (select address_id from found_user)
), risk_profile as (
    select risk_id, level_id
    from addr, jsonb_to_recordset(profile) x(risk_id int, level_id int)
), risk_subscriptions as (
    select id, event_id
    from events_subscriptions
	where user_id = (select id from found_user)
), risk_ids as (
	select i.risk_id, level_id, total_points
	from (
		select
        	ec.event_id as risk_id,
        	sum(c.readiness_points) as total_points
     	from chapters c
     	join event_chapters ec on ec.chapter_id = c.id
     	where ec.event_id in (select event_id from risk_subscriptions)
     	group by ec.event_id
	) i
	left join risk_profile rp on rp.risk_id = i.risk_id
	where i.risk_id not in (2, 5) and level_id >= 3
), high_count as (
	select count(*)
	from risk_ids
	where level_id >= 4
), med_count as (
	select count(*)
	from risk_ids
	where level_id >= 3 and risk_id != 4
), ordered_risk_ids as (
	select case
	when (select count from high_count) >= 2 then (
		select array_agg(risk_id)
		from (
			select risk_id
			from risk_ids
			where level_id >= 4
			order by level_id desc, total_points desc
		) a1
	)
	when (select count from high_count) = 1 and (select count from med_count) >= 1 then (
		select array_agg(risk_id)
		from (
			(select risk_id from risk_ids where level_id >= 4)
			union (select risk_id from risk_ids where level_id = 3 order by total_points desc limit 1)
		) a1
	)
	when (select count from med_count) >= 1 then (
		select format(
			'{4,%s}',
			(
				select risk_id
			 	from risk_ids
			 	where level_id = 3 and risk_id != 4
			 	order by total_points desc
			 	limit 1
			)
		)::int[]
	)
	else (select '{4,1}'::int[])
	end as ids
), risks_json as (
	select array_to_json(ids) as risks
	from ordered_risk_ids
), risk_objects as (
	select array_to_json(array_agg(json_build_object('id', value, 'type', 'risk'))) as risk_objects
	from json_array_elements((select risks from risks_json))
), activity_ids as (
    select id
    from activities
    where theme_id in (select id from activity_themes)
), activity_points as (
    select
        sum(c.readiness_points) as total,
        c.activity_id as id
    from chapters c
    inner join event_chapters ec on ec.chapter_id = c.id
    inner join risk_subscriptions s on s.event_id = ec.event_id
    where c.activity_id in (select id from activity_ids)
    group by c.activity_id
), theme_ids as (
	select theme_id from (
		select
        	a.theme_id as theme_id,
         	sum(total) as total
	    from activity_points ap
     	join activities a on a.id = ap.id
    	group by a.theme_id
	) i
	order by total desc
), themes_json as (
	select coalesce(array_to_json(array_agg(theme_id)), '[5]') as themes
	from theme_ids
), theme_objects as (
	select array_to_json(array_agg(json_build_object('id', value, 'type', 'theme'))) as theme_objects
	from json_array_elements((select themes from themes_json))
)
insert into weekly_schedules (user_id, schedule)
select
	(select id as user_id from found_user),
	(select risk_objects from risk_objects)::jsonb
	|| (select theme_objects from theme_objects)::jsonb as schedule
on conflict (user_id) do update set schedule = excluded.schedule`
