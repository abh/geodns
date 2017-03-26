package geoip2

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strings"
	"sync"

	"github.com/abh/geodns/countries"
	"github.com/abh/geodns/targeting/geo"
	geoip2 "github.com/oschwald/geoip2-golang"
)

type geoType uint8

const (
	countryDB = iota
	cityDB
	asnDB
)

type g2 struct {
	dir string

	country *geoip2.Reader
	city    *geoip2.Reader
	asn     *geoip2.Reader
	mu      sync.RWMutex
}

func (g *g2) open(t geoType, db string) (*geoip2.Reader, error) {
	f := filepath.Join(g.dir, db)
	n, err := geoip2.Open(f)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	switch t {
	case countryDB:
		g.country = n
	case cityDB:
		g.city = n
	case asnDB:
		g.asn = n
	}
	return n, nil
}

func (g *g2) get(t geoType, db string) (*geoip2.Reader, error) {
	g.mu.RLock()

	var r *geoip2.Reader

	switch t {
	case countryDB:
		r = g.country
	case cityDB:
		r = g.city
	case asnDB:
		r = g.asn
	}

	// unlock so the g.open() call below won't lock
	g.mu.RUnlock()

	if r != nil {
		return r, nil
	}

	return g.open(t, db)
}

func New(dir string) (*g2, error) {
	g := &g2{
		dir: dir,
	}
	_, err := g.open(countryDB, "GeoIP2-Country.mmdb")
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *g2) HasASN() (bool, error) {
	r, err := g.get(asnDB, "GeoIP2-ASN.mmdb")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

func (g *g2) GetASN(ip net.IP) (string, int, error) {
	r, err := g.get(asnDB, "GeoIP2-ASN.mmdb")
	if err != nil {
		return "", 0, err
	}

	c, err := r.ISP(ip)
	if err != nil {
		return "", 0, fmt.Errorf("lookup ASN for '%s': %s", ip.String(), err)
	}
	asn := c.AutonomousSystemNumber
	return fmt.Sprintf("as%d", asn), 0, nil
}

func (g *g2) HasCountry() (bool, error) {
	r, err := g.get(countryDB, "GeoIP2-Country.mmdb")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

func (g *g2) GetCountry(ip net.IP) (country, continent string, netmask int) {
	r, err := g.get(countryDB, "GeoIP2.mmdb")
	c, err := r.Country(ip)
	if err != nil {
		log.Printf("Could not lookup country for '%s': %s", ip.String(), err)
		return "", "", 0
	}

	country = c.Country.IsoCode

	if len(country) > 0 {
		country = strings.ToLower(country)
		continent = countries.CountryContinent[country]
	}

	return country, continent, 0
}

func (g *g2) HasLocation() (bool, error) {
	r, err := g.get(cityDB, "GeoIP2-City.mmdb")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

func (g *g2) GetLocation(ip net.IP) (l *geo.Location, err error) {
	c, err := g.city.City(ip)
	if err != nil {
		log.Printf("Could not lookup CountryRegion for '%s': %s", ip.String(), err)
		return
	}

	l = &geo.Location{
		Latitude:  float64(c.Location.Latitude),
		Longitude: float64(c.Location.Longitude),
		Country:   strings.ToLower(c.Country.IsoCode),
	}

	if len(c.Subdivisions) > 0 {
		l.Region = strings.ToLower(c.Subdivisions[0].IsoCode)
	}
	if len(l.Country) > 0 {
		l.Continent = countries.CountryContinent[l.Country]
		if len(l.Region) > 0 {
			l.Region = l.Country + "-" + l.Region
			l.RegionGroup = countries.CountryRegionGroup(l.Country, l.Region)
		}
	}

	return

}
