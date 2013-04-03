package main

import (
	"github.com/abh/geoip"
	"log"
)

func setupGeoIP() *geoip.GeoIP {

	gi, err := geoip.Open()
	if gi == nil || err != nil {
		log.Printf("Could not open GeoIP database: %s\n", err)
		return nil
	}
	return gi
}
