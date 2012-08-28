package main

import (
	"github.com/abh/geoip"
	"log"
)

func setupGeoIP() *geoip.GeoIP {

	gi := geoip.Open()
	if gi == nil {
		log.Printf("Could not open GeoIP database\n")
		return nil
	}
	return gi
}
