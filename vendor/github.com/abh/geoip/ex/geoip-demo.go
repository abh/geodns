package main

import (
	"fmt"
	"github.com/abh/geoip"
)

func main() {

	file6 := "../db/GeoIPv6.dat"

	gi6, err := geoip.Open(file6)
	if err != nil {
		fmt.Printf("Could not open GeoIPv6 database: %s\n", err)
	}

	gi, err := geoip.Open()
	if err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
	}

	giasn, err := geoip.Open("../db/GeoIPASNum.dat")
	if err != nil {
		fmt.Printf("Could not open GeoIPASN database: %s\n", err)
	}

	giasn6, err := geoip.Open("../db/GeoIPASNumv6.dat")
	if err != nil {
		fmt.Printf("Could not open GeoIPASN database: %s\n", err)
	}

	if giasn != nil {
		ip := "207.171.7.51"
		asn, netmask := giasn.GetName(ip)
		fmt.Printf("%s: %s (netmask /%d)\n", ip, asn, netmask)

	}

	if gi != nil {
		test4(*gi, "207.171.7.51")
		test4(*gi, "127.0.0.1")
	}
	if gi6 != nil {
		ip := "2607:f238:2::5"
		country, netmask := gi6.GetCountry_v6(ip)
		var asn string
		var asn_netmask int
		if giasn6 != nil {
			asn, asn_netmask = giasn6.GetNameV6(ip)
		}
		fmt.Printf("%s: %s/%d %s/%d\n", ip, country, netmask, asn, asn_netmask)

	}

}

func test4(g geoip.GeoIP, ip string) {
	test(func(s string) (string, int) { return g.GetCountry(s) }, ip)
}

func test(f func(string) (string, int), ip string) {
	country, netmask := f(ip)
	fmt.Printf("ip: %s is [%s] (netmask %d)\n", ip, country, netmask)

}
