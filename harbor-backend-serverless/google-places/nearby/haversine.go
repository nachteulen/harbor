package main

import (
	"math"
)

const earthRadiusMiles = 3958.8

func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

func milesBetween(lat1, lon1, lat2, lon2 float64) float64 {
	var la1, lo1, la2, lo2 float64
	la1 = lat1 * math.Pi / 180
	lo1 = lon1 * math.Pi / 180
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * earthRadiusMiles * math.Asin(math.Sqrt(h))
}
