package maxmind

import (
	mx "github.com/oschwald/maxminddb-golang"
)

var (
	mDB *mx.Reader
)

func initMxDB() error {
	if mDB != nil {
		return nil
	}

	db, err := mx.Open("/mnt/efs/GeoIP2-City.mmdb")
	if err != nil {
		return err
	}

	mDB = db
	return nil
}
