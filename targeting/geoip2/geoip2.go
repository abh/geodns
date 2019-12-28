package geoip2

import (
	"fmt"
	"log"
	"net"
	"os"
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

var dbFiles map[geoType][]string

// GeoIP2 contains the geoip implementation of the GeoDNS geo
// targeting interface
type GeoIP2 struct {
	dir string

	country *geoip2.Reader
	city    *geoip2.Reader
	asn     *geoip2.Reader
	mu      sync.RWMutex
}

func init() {
	dbFiles = map[geoType][]string{
		countryDB: []string{"GeoIP2-Country.mmdb", "GeoLite2-Country.mmdb"},
		asnDB:     []string{"GeoIP2-ASN.mmdb", "GeoLite2-ASN.mmdb"},
		cityDB:    []string{"GeoIP2-City.mmdb", "GeoLite2-City.mmdb"},
	}
}

// FindDB returns a guess at a directory path for GeoIP data files
func FindDB() string {
	dirs := []string{
		"/usr/share/GeoIP/",       // Linux default
		"/usr/share/local/GeoIP/", // source install?
		"/usr/local/share/GeoIP/", // FreeBSD
		"/opt/local/share/GeoIP/", // MacPorts
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			if os.IsExist(err) {
				log.Println(err)
			}
			continue
		}
		return dir
	}
	return ""
}

func (g *GeoIP2) open(t geoType, db string) (*geoip2.Reader, error) {

	fileName := filepath.Join(g.dir, db)

	if len(db) == 0 {
		found := false
		for _, f := range dbFiles[t] {
			fileName = filepath.Join(g.dir, f)
			if _, err := os.Stat(fileName); err == nil {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("could not find '%s' in '%s'", dbFiles[t], g.dir)
		}
	}

	n, err := geoip2.Open(fileName)
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

func (g *GeoIP2) get(t geoType, db string) (*geoip2.Reader, error) {
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

// New returns a new GeoIP2 provider
func New(dir string) (*GeoIP2, error) {
	g := &GeoIP2{
		dir: dir,
	}
	_, err := g.open(countryDB, "")
	if err != nil {
		return nil, err
	}

	return g, nil
}

// HasASN returns if we can do ASN lookups
func (g *GeoIP2) HasASN() (bool, error) {
	r, err := g.get(asnDB, "")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

// GetASN returns the ASN for the IP (as a "as123" string) and the netmask
func (g *GeoIP2) GetASN(ip net.IP) (string, int, error) {
	r, err := g.get(asnDB, "")
	log.Printf("GetASN for %s, got DB? %s", ip, err)
	if err != nil {
		return "", 0, err
	}

	c, err := r.ASN(ip)
	if err != nil {
		return "", 0, fmt.Errorf("lookup ASN for '%s': %s", ip.String(), err)
	}
	asn := c.AutonomousSystemNumber
	netmask := 24
	if ip.To4() != nil {
		netmask = 48
	}
	return fmt.Sprintf("as%d", asn), netmask, nil
}

// HasCountry checks if the GeoIP country database is available
func (g *GeoIP2) HasCountry() (bool, error) {
	r, err := g.get(countryDB, "")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

// GetCountry returns the country, continent and netmask for the given IP
func (g *GeoIP2) GetCountry(ip net.IP) (country, continent string, netmask int) {
	r, err := g.get(countryDB, "")
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

// HasLocation returns if the city database is available to
// return lat/lon information for an IP
func (g *GeoIP2) HasLocation() (bool, error) {
	r, err := g.get(cityDB, "")
	if r != nil && err == nil {
		return true, nil
	}
	return false, err
}

// GetLocation returns a geo.Location object for the given IP
func (g *GeoIP2) GetLocation(ip net.IP) (l *geo.Location, err error) {
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
