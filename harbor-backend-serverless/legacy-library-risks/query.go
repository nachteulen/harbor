package main

// TODO: the regex filter is annoying, the address should just point to a
// zipcode_location, which would validate the FK at insertion time...
var query = `
with location as (
    select
        a.zipcode,
        a.latitude,
        a.longitude,
        zl.state_abbr
    from users u
    join addresses a on u.address_id = a.id
    left join zipcode_locations zl on zl.id = a.zipcode::integer
    where u.id = $1 and a.zipcode ~ E'^\\d+$'
),
subscriptions as (
    select id, event_id
    from events_subscriptions
    where ownership_id in ($2, $3)
),
event_points_cte as (
    select
        sum(c.readiness_points) as total,
        sum(case when cc.id is not null then c.readiness_points else 0 end) as current,
        ec.event_id
    from chapters c
    left join completed_chapters cc on cc.chapter_id = c.id and cc.ownership_id in ($4,$5)
    inner join event_chapters ec on ec.chapter_id = c.id
    group by ec.event_id
),
events as (
    select
        e.id,
        e.name,
        e.disclaimer,
        list_image.path as list_image_path,
        icon_image.path as icon_image_path,
        is_priority,
        coalesce(case
            when event_points_cte.total = 0 then 0
            else event_points_cte.current::decimal/event_points_cte.total
            end, 0) as readiness,
        case when s.id is not null then true else false end as is_subscribed,
        (select zipcode from location),
        (select latitude from location),
        (select longitude from location),
        (select state_abbr from location)
    from events e
    left join subscriptions s on s.event_id = e.id
    left join event_points_cte on event_points_cte.event_id = e.id
    left join files list_image on list_image.uuid = e.event_list_image_uuid
    left join files icon_image on icon_image.uuid = e.icon_uuid
    where e.enabled = true
) select * from events order by case when is_subscribed then 1 end`
