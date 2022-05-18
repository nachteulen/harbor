package lib

import (
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const query = `
with user_data as (
	select
		a.latitude as alat,
		a.longitude as alng,
		zl.latitude as zlat,
		zl.longitude as zlng
	from users u
	join addresses a on a.id = u.address_id
	left join zipcode_locations zl on zl.id = a.zipcode::int
	where u.id = $1
)

select lat, lng from (
	select alat as lat, alng as lng, 'addr' as type
	from user_data
	union
	select zlat as lat, zlng as lng, 'zip' as type
	from user_data
) o1
where lat is not null and lng is not null
order by case when type = 'addr' then 0 else 1 end
limit 1`

var (
	pgDB *sqlx.DB
)

func initPgDB() error {
	if pgDB != nil {
		return nil
	}

	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		return err
	}

	pgDB = d
	return nil
}
