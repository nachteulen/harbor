package maxmind

import (
	"fmt"
	"net"
)

type GeoResponse struct {
	City      *string  `json:"city"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Zipcode   *string  `json:"zipcode"`
	StateAbbr *string  `json:"stateAbbr"`
}

type DBRecord struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Lat *float64 `maxminddb:"latitude"`
		Lng *float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Postal struct {
		Code *string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Subdivisions []struct {
		IsoCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

func GetLocationFromIP(sIP string) (*GeoResponse, error) {
	if err := initMxDB(); err != nil {
		return nil, fmt.Errorf("mxDB initialization error: %s", err)
	}

	var dbRecord DBRecord
	if err := mDB.Lookup(net.ParseIP(sIP), &dbRecord); err != nil {
		return nil, fmt.Errorf("unable to lookup ip(%s): %s", sIP, err)
	}

	if dbRecord.Country.IsoCode != "US" {
		return nil, fmt.Errorf("non US ip(%s): %s", sIP, dbRecord.Country.IsoCode)
	}

	resp := GeoResponse{
		Latitude:  dbRecord.Location.Lat,
		Longitude: dbRecord.Location.Lng,
		Zipcode:   dbRecord.Postal.Code,
	}

	city, ok := dbRecord.City.Names["en"]
	if ok {
		resp.City = &city
	}

	if len(dbRecord.Subdivisions) != 0 {
		stateAbbr := dbRecord.Subdivisions[0].IsoCode
		if len(stateAbbr) != 0 {
			resp.StateAbbr = &stateAbbr
		}
	}

	return &resp, nil
}
