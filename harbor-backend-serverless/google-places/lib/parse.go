package lib

import (
	"database/sql"
	"fmt"

	mx "github.com/helloharbor/harbor-backend-serverless/maxmind"
)

type CoordinatePair struct {
	Lat float64
	Lng float64
}

func getCurrent(sIP string) (*CoordinatePair, error) {
	record, err := mx.GetLocationFromIP(sIP)
	if err != nil {
		return nil, fmt.Errorf("error getting current location: %s", err)
	}

	if record.Latitude == nil || record.Longitude == nil {
		return nil, fmt.Errorf("invalid coordinates(%+v) for ip(%s)", record, sIP)
	}
	return &CoordinatePair{*record.Latitude, *record.Longitude}, nil
}

func ParseOrigin(userID, origin, sIP string) (*CoordinatePair, error) {
	if origin == "current" {
		return getCurrent(sIP)
	}

	if origin == "home" {
		if err := initPgDB(); err != nil {
			return nil, fmt.Errorf("db initialization error: %s", err)
		}

		var home struct {
			Lat float64 `db:"lat"`
			Lng float64 `db:"lng"`
		}
		err := pgDB.Get(&home, query, userID)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("no address for user(%s)\n", userID)
			} else {
				fmt.Printf("error getting address for user(%s)\n", userID)
			}
			// fallback to current location
			return getCurrent(sIP)
		}

		return &CoordinatePair{home.Lat, home.Lng}, nil
	}
	return nil, fmt.Errorf("unrecognized origin: %s", origin)
}
