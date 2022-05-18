package main

var query = `
with input as (
	select id, type, is_global, data_type
	from form_inputs
	where id = $1
), all_options as (
	select meta -> 'options'
	from form_inputs
	where id = (select id from input)
), filtered_options as (
	select value ->> 'value' as value, value ->> 'points' as points
	from jsonb_array_elements((select * from all_options))
	where value != 'null'
), multiselect_target as (
	select points::integer
	from filtered_options
	where value = $2
), multiselect_points as (
	select case
	when exists(select points from multiselect_target)
		then (select points from multiselect_target)
	else 0 end as points
), points as (
	select case
	when (select type from input) = 'multiselect'
		then (select points from multiselect_points)
	when (select type from input) = 'capture'
		then 1
	else
		0
	end as points
), args as (
	select
		(select case when (select is_global from input) = true
			then null else $3 end) as plan_id,
		(select case when (select is_global from input) = true
			then null else $4 end) as form_id
), user_answer as (
	select $5 as value
), validated as (
	select case
		when (select data_type from input) = 'integer'
			then (select value from user_answer)::integer::text
		when (select data_type from input) = 'date'
			then (select value from user_answer)::date::text
		else
			(select value from user_answer)
		end as value
), plan as (
    select
    	id,
    	name,
    	max_points,
    	case
            when o1.max_version_key is null then form_ids
            else form_ids->o1.max_version_key end as form_ids
    from plans p
    left join lateral (
        select max(version_key)::text as max_version_key
        from (select jsonb_object_keys(form_ids)::int as version_key) o2
        where version_key <= $6 and jsonb_typeof(form_ids) = 'object'
    ) o1 on true
    where id = $7
), current_plan_points as (
    select sum(coalesce(fia.points, 0)) as points
    from plan p
    join forms f on f.id in (
        select fid::int from (select jsonb_array_elements(p.form_ids) as fid) fids
    )
    join form_inputs fi on fi.id in (
        select in_id::int from (select jsonb_array_elements(input_ids) as in_id) in_ids
    )
    left join form_input_answers fia on case
        when fi.is_global = true then fia.input_id = fi.id
        else (fia.plan_id = p.id and fia.form_id = f.id and fia.input_id = fi.id) end
        and fia.household_id = $8
), inserted as (
	insert into form_input_answers
	(
		plan_id,
		form_id,
		input_id,
		household_id,
		value,
		points,
		user_id,
		meta
	) values (
		(select plan_id::int from args),
		(select form_id::int from args),
		(select id from input),
		$9,
		(select value from validated),
		(select points from points),
		$10,
		$11
	)
	on conflict do nothing
	returning id
), answer_id as (
	select case
	when (select id from inserted) is not null then (select id from inserted)
	when (select is_global from input) = true then (
		select id
		from form_input_answers
		where input_id = (select id from input) and household_id = $12
	)
	else (
		select id
		from form_input_answers
		where
			plan_id = (select plan_id::int from args)
			and form_id = (select form_id::int from args)
			and input_id = (select id from input)
			and household_id = $13
	)
	end as id
)
select
	(select id from answer_id) as answer_id,
	(select points from current_plan_points) as current_plan_points,
	(select points from points) as added_points,
	(select max_points from plan) as max_points,
	(select name from plan) as plan_name`
