package targeting

import (
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/abh/geodns/countries"

	"github.com/abh/geoip"
	"github.com/golang/geo/s2"
)

type GeoIPData struct {
	country         *geoip.GeoIP
	hasCountry      bool
	countryLastLoad time.Time

	city         *geoip.GeoIP
	cityLastLoad time.Time
	hasCity      bool

	asn         *geoip.GeoIP
	hasAsn      bool
	asnLastLoad time.Time
}

const MAX_DISTANCE = 360

type Location struct {
	Latitude  float64
	Longitude float64
}

func (l *Location) MaxDistance() float64 {
	return MAX_DISTANCE
}

func (l *Location) Distance(to *Location) float64 {
	if to == nil {
		return MAX_DISTANCE
	}
	ll1 := s2.LatLngFromDegrees(l.Latitude, l.Longitude)
	ll2 := s2.LatLngFromDegrees(to.Latitude, to.Longitude)
	angle := ll1.Distance(ll2)
	return math.Abs(angle.Degrees())
}

var geoIP = &GeoIPData{}

func GeoIP() *GeoIPData {
	// mutex this and allow it to reload as needed?
	return geoIP
}

func (g *GeoIPData) GetCountry(ip net.IP) (country, continent string, netmask int) {
	if g.country == nil {
		return "", "", 0
	}

	country, netmask = g.country.GetCountry(ip.String())
	if len(country) > 0 {
		country = strings.ToLower(country)
		continent = countries.CountryContinent[country]
	}
	return
}

func (g *GeoIPData) GetCountryRegion(ip net.IP) (country, continent, regionGroup, region string, netmask int, location *Location) {
	if g.city == nil {
		log.Println("No city database available")
		country, continent, netmask = g.GetCountry(ip)
		return
	}

	record := g.city.GetRecord(ip.String())
	if record == nil {
		return
	}

	location = &Location{float64(record.Latitude), float64(record.Longitude)}

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

func (g *GeoIPData) GetASN(ip net.IP) (asn string, netmask int) {
	if g.asn == nil {
		log.Println("No asn database available")
		return
	}
	name, netmask := g.asn.GetName(ip.String())
	if len(name) > 0 {
		index := strings.Index(name, " ")
		if index > 0 {
			asn = strings.ToLower(name[:index])
		}
	}
	return
}

func (g *GeoIPData) SetDirectory(directory string) {
	// directory := Config.GeoIPDataDirectory()
	if len(directory) > 0 {
		geoip.SetCustomDirectory(directory)
	}
}

func (g *GeoIPData) SetupGeoIPCountry() {
	if g.country != nil {
		return
	}

	gi, err := geoip.OpenType(geoip.GEOIP_COUNTRY_EDITION)
	if gi == nil || err != nil {
		log.Printf("Could not open country GeoIPData database: %s\n", err)
		return
	}
	g.countryLastLoad = time.Now()
	g.hasCity = true
	g.country = gi

}

func (g *GeoIPData) SetupGeoIPCity() {
	if g.city != nil {
		return
	}

	gi, err := geoip.OpenType(geoip.GEOIP_CITY_EDITION_REV1)
	if gi == nil || err != nil {
		log.Printf("Could not open city GeoIPData database: %s\n", err)
		return
	}
	g.cityLastLoad = time.Now()
	g.hasCity = true
	g.city = gi

}

func (g *GeoIPData) SetupGeoIPASN() {
	if g.asn != nil {
		return
	}

	gi, err := geoip.OpenType(geoip.GEOIP_ASNUM_EDITION)
	if gi == nil || err != nil {
		log.Printf("Could not open ASN GeoIPData database: %s\n", err)
		return
	}
	g.asnLastLoad = time.Now()
	g.hasAsn = true
	g.asn = gi

}
