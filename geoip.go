package main

import (
	"fmt"
	"github.com/abh/geoip"
)

func setupGeoIP() *geoip.GeoIP {
	file := "/opt/local/share/GeoIP/GeoIP.dat"

	gi := geoip.GeoIP_Open(file)
	if gi == nil {
		fmt.Printf("Could not open GeoIP database\n")
		return nil
	}
	return gi
}
