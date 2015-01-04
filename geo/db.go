package geo

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/abh/geoip"

	"github.com/abh/geodns/countries"
)

type geodb struct {
	db       *geoip.GeoIP
	loaded   bool
	lastLoad time.Time
}

type geodbs struct {
	isv6    bool
	country geodb
	city    geodb
	asn     geodb
}

func (g *geodbs) GetCountry(ip net.IP) (country, continent string, netmask int) {
	if !g.country.loaded {
		return "", "", 0
	}

	country, netmask = g.country.db.GetCountry(ip.String())
	if len(country) > 0 {
		country = strings.ToLower(country)
		continent = countries.CountryContinent[country]
	}
	return
}

func (g *geodbs) GetCountryRegion(ip net.IP) (country, continent, regionGroup, region string, netmask int) {
	if !g.city.loaded {
		log.Println("No city database available")
		country, continent, netmask = g.GetCountry(ip)
		return
	}

	record := g.city.db.GetRecord(ip.String())
	if record == nil {
		return
	}

	country = record.CountryCode
	region = record.Region
	if len(country) > 0 {
		country = strings.ToLower(country)
		continent = countries.CountryContinent[country]

		if len(region) > 0 {
			region = country + "-" + strings.ToLower(region)
			regionGroup = countries.CountryRegionGroup(country, region)
		}

	}
	return
}

func (g *geodbs) GetASN(ip net.IP) (asn string, netmask int) {
	if !g.asn.loaded {
		log.Println("No asn database available")
		return
	}
	asn, netmask = g.asn.db.GetName(ip.String())
	if len(asn) > 0 {
		index := strings.Index(asn, " ")
		if index > 0 {
			asn = strings.ToLower(asn[:index])
		}
	}
	return
}
