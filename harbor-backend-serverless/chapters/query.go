package main

const query = `
with ownerships as (
    select oid::int
    from (select jsonb_array_elements($1) as oid) oids
), chapter as (
	select
		c.id,
		c.name,
		c.description,
		(select exists(
			select id
			from completed_chapters
			where
				chapter_id = $2
				and ownership_id in (select oid from ownerships)
		)) as is_completed
	from chapters c
	where c.id = $3
), steps as (
	select
		s.id as step_id,
		s.name as step_name,
		s.raw_value as raw_step,
		s.step_data_type_id,
		s.is_optional,
		s.inventory_ids,
		case when cs.id is null then false else true end as is_completed,
		a.id as answer_id,
		a.name as answer_name,
		a.raw_value as raw_answer
	from steps s
	left join completed_steps cs on
		cs.step_id = s.id
		and cs.ownership_id in (select oid from ownerships)
	left join answers a on
		a.step_id = s.id
		and a.ownership_id in (select oid from ownerships)
	where s.chapter_id = (select id from chapter)
), steps_json as (
	select array_to_json(array_agg(json_build_object(
		'stepID', step_id,
		'stepName', step_name,
		'rawStep', raw_step,
		'stepDataTypeID', step_data_type_id,
		'isOptional', is_optional,
		'inventoryIDs', inventory_ids,
		'isCompleted', is_completed,
		'answerID', answer_id,
		'answerName', answer_name,
		'rawAnswer', raw_answer
	)))
	from steps
)
select json_build_object(
	'id', id,
	'name', name,
	'description', description,
	'isCompleted', is_completed,
	'steps', (select * from steps_json)
) from chapter`
