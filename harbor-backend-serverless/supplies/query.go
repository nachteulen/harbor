package main

const query = `
with ownerships as (
    select oid::int
    from (select jsonb_array_elements($1) as oid) oids
), uci as (
	select id, inventory_category_id, owned
	from user_custom_inventories
	where ownership_id in (select oid from ownerships)
), all_counts as (
	select inventory_category_id, count(*) as all_count
	from (
		select id, inventory_category_id, 'default' as type
		from inventories
		union
		select id, inventory_category_id, 'custom' as type
		from uci
	) o1
	group by 1
), owned_inventories as (
	select distinct on (inventory_id) oi.id, inventory_category_id
	from ownership_inventories oi
	join inventories i on i.id = oi.inventory_id
	where ownership_id in (select oid from ownerships)
), owned_counts as (
	select inventory_category_id, count(*) as owned_count
	from (
		select id, inventory_category_id, 'default' as type
		from owned_inventories
		union
		select id, inventory_category_id, 'custom' as type
		from uci
		where owned = true
	) o1
	group by 1
), supplies_summary_json as (
	select array_to_json(array_agg(json_build_object(
		'id', ac.inventory_category_id,
		'name', ic.name,
		'allItemsCount', coalesce(all_count, 0),
		'ownedItemsCount', coalesce(owned_count, 0)
	))) as summary
	from all_counts ac
	left join owned_counts oc on oc.inventory_category_id = ac.inventory_category_id
	join inventory_categories ic on ic.id = ac.inventory_category_id
)
select json_build_object('summary', summary)
from supplies_summary_json`
