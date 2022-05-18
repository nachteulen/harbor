package main

const query = `
with answer as (
	select id, household_id, input_id
	from form_input_answers
	where id = $1 and household_id = $2
), input as (
	select type, data_type
	from form_inputs
	where id = (select input_id from answer)
), all_options as (
	select meta -> 'options'
	from form_inputs
	where id = (select input_id from answer)
), filtered_options as (
	select value ->> 'value' as value, value ->> 'points' as points
	from jsonb_array_elements((select * from all_options))
	where value != 'null'
), multiselect_target as (
	select points::integer
	from filtered_options
	where value = $3
), multiselect_points as (
	select case
	when exists(select points from multiselect_target)
		then (select points from multiselect_target)
	else 0 end as points
), user_answer as (
	select $4 as value
), points as (
	select case
	when (select type from input) = 'multiselect'
		then (select points from multiselect_points)
	when (select type from input) = 'capture' and length((select value from user_answer)) != 0
		then 1
	else
		0
	end as points
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
        where version_key <= $5 and jsonb_typeof(form_ids) = 'object'
    ) o1 on true
    where id = $6
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
        and fia.household_id = $7
), plan_progress as (
	select (
		(select points from current_plan_points)
		+ (select points from points)) / (select max_points::real from plan) as progress
), validated as (
	select case
		when (select data_type from input) = 'integer'
			then (select value from user_answer)::integer::text
		when (select data_type from input) = 'date' and length((select value from user_answer)) != 0
			then (select value from user_answer)::date::text
		else
			(select value from user_answer)
		end as value
), updates as (
	update form_input_answers
	set
		value = (select value from validated),
		points = (select points from points),
		user_id = $8,
		meta = $9
	where id = (select id from answer)
	and household_id = (select household_id from answer)
)
select
	(select points from current_plan_points) as current_plan_points,
	(select points from points) as added_points,
	(select max_points from plan) as max_points,
	(select name from plan) as plan_name`
