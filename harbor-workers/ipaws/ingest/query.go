package main

const ugcQuery = `
select
	convex_hull,
	bb_lat_lo,
	bb_lat_hi,
	bb_lng_lo,
	bb_lng_hi
from geocode_ugc
where
	ugc_code = $1
`

type GeocodeUgcRow struct {
	ConvexHull string  `db:"convex_hull"`
	BBLatLo    float64 `db:"bb_lat_lo"`
	BBLatHi    float64 `db:"bb_lat_hi"`
	BBLngLo    float64 `db:"bb_lng_lo"`
	BBLngHi    float64 `db:"bb_lng_hi"`
}
