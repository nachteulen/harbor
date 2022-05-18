package main

const query = `
with user_data as (
	select u.id, rp.id as risk_profile_id
	from users u
	left join addresses a on a.id = u.address_id
	left join risk_profiles rp on rp.id = a.risk_profile_id
	where u.id = $1
), current_household as (
	select current_household.id
	from users u
	inner join household_users owner_hu on
		u.id = owner_hu.user_id and owner_hu.household_user_type_id = 1
	left join household_users invited_hu on
		u.id = invited_hu.user_id and invited_hu.household_user_type_id in (2, 4, 5)
	inner join households current_household on
		coalesce(invited_hu.household_id, owner_hu.household_id) = current_household.id
	where u.id = (select id from user_data)
), local_authorities as (
	select
		now() as created_at,
		coalesce(local_authorities::json, '{}') as meta,
		'local_authorities' as row_type
	from risk_profiles
	where id = (select risk_profile_id from user_data)
), members as (
	select
		hu.created_at,
		json_build_object(
			'id', hu.id,
			'householdUserTypeID', household_user_type_id,
			'firstName', hu.first_name,
			'dateOfBirth', date_of_birth,
			'healthInformation', health_information,
			'phone', phone,
			'avatarID', hu.avatar_id,
			'user', json_build_object(
				'id', u.id,
				'email', u.email,
				'socialEmail', u.social_email,
				'firstName', u.first_name,
				'lastName', u.last_name,
				'phoneNumber', u.phone_number
			)
		) as meta,
		'member' as row_type
	from household_users hu
	left join users u on u.id = hu.user_id
	where household_id = (select id from current_household)
), emergency_contacts as (
	select
		ec.created_at,
		json_build_object(
		    'id', ec.id,
		    'emergencyContactTypeID', emergency_contact_type_id,
			'address', json_build_object(
				'address', a.address,
				'latitude', a.latitude,
				'longitude', a.longitude
			),
			'contactTypeDescription', contact_type_description,
			'email', email,
			'fullName', full_name,
			'phoneNumber', phone_number
		) as meta,
		'contact' as row_type
	from emergency_contacts ec
	left join addresses a on a.id = ec.address_id
	left join zipcode_locations zl on zl.id = a.zipcode::integer
	where user_id = (select id from user_data)
), unioned as (
	select * from local_authorities
	union all
	select * from members
	union all
	select * from emergency_contacts
	order by 3,1 desc
)
select meta, row_type
from unioned`
