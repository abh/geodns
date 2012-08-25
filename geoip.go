package main

import (
	"github.com/abh/geoip"
	"log"
)

func setupGeoIP() *geoip.GeoIP {
	file := "/opt/local/share/GeoIP/GeoIP.dat"

	gi := geoip.GeoIP_Open(file)
	if gi == nil {
		log.Printf("Could not open GeoIP database\n")
		return nil
	}
	return gi
}
