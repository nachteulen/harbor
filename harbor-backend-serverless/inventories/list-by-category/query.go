package main

const query = `
with ownerships as (
    select oid::int
    from (select jsonb_array_elements($1) as oid) oids
), category_information as (
	select
		ic.id,
		ic.name,
		case when ici.id is not null then json_build_object(
			'id', ici.id,
			'location', ici.storage_location,
			'fileUUID', file_uuid
		) else null end as userStorage
	from inventory_categories ic
	left join
		inventory_category_information ici on ici.inventory_category_id = ic.id
		and ici.ownership_id in (select oid from ownerships)
	where ic.id = $2
), category_json as (
	select json_build_object(
		'id', id,
		'name', name,
		'userStorage', userStorage
	) as data
	from category_information
), inventory_items as (
	select
		i.id,
		i.name,
		i.description,
		i.section_name,
		i.product_links::text,
		case when oi.id is null then false else true end as owned,
		false as is_custom_inventory
	from inventories i
	left join ownership_inventories oi on oi.inventory_id = i.id and oi.ownership_id in (select oid from ownerships)
	where i.inventory_category_id = (select id from category_information)
), custom_inventory_items as (
	select
		id,
		name,
		null,
		null,
		null,
		owned,
		true
	from user_custom_inventories
	where
		ownership_id in (select oid from ownerships)
		and inventory_category_id = (select id from category_information)
), inventory_items_json as (
	select json_agg(json_build_object(
		'id', id,
		'name', name,
		'description', description,
		'sectionName', section_name,
		'productLinks', coalesce(product_links, '[]')::jsonb,
		'owned', owned,
		'isCustomInventory', is_custom_inventory
	)) as items
	from (
		select * from inventory_items
		union
		select * from custom_inventory_items
		order by id
	) i
)
select
	(select data from category_json) as category_json,
	(select items from inventory_items_json) as inventory_items_json`
