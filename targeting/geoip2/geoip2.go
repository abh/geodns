package geoip2

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abh/geodns/v3/countries"
	"github.com/abh/geodns/v3/targeting/geo"
	gdb "github.com/oschwald/geoip2-golang"
)

// GeoIP2 contains the geoip implementation of the GeoDNS geo
// targeting interface
type GeoIP2 struct {
	dir     string
	country geodb
	city    geodb
	asn     geodb
}

type geodb struct {
	active       bool
	lastModified int64        // Epoch time
	fp           string       // FilePath
	db           *gdb.Reader  // Database reader
	l            sync.RWMutex // Individual lock for separate DB access and reload -- Future?
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

// open will create a filehandle for the provided GeoIP2 database. If opened once before and a newer modification time is present, the function will reopen the file with its new contents
func (g *GeoIP2) open(v *geodb, fns ...string) error {
	var fi fs.FileInfo
	var err error
	if v.fp == "" {
		// We're opening this file for the first time
		for _, i := range fns {
			fp := filepath.Join(g.dir, i)
			fi, err = os.Stat(fp)
			if err != nil {
				continue
			}
			v.fp = fp
		}
	}
	if v.fp == "" { // Recheck for empty string in case none of the DB files are found
		return fmt.Errorf("no files found for db")
	}
	if fi == nil { // We have not set fileInfo and v.fp is set
		fi, err = os.Stat(v.fp)
	}
	if err != nil {
		return err
	}
	if v.lastModified >= fi.ModTime().UTC().Unix() { // No update to existing file
		return nil
	}
	// Delay the lock to here because we're only
	v.l.Lock()
	defer v.l.Unlock()

	o, e := gdb.Open(v.fp)
	if e != nil {
		return e
	}
	v.db = o
	v.active = true
	v.lastModified = fi.ModTime().UTC().Unix()

	return nil
}

// watchFiles spawns a goroutine to check for new files every minute, reloading if the modtime is newer than the original file's modtime
func (g *GeoIP2) watchFiles() {
	// Not worried about goroutines leaking because only one geoip2.New call is made in main (outside of testing)
	ticker := time.NewTicker(1 * time.Minute)
	for { // We forever-loop here because we only run this function in a separate goroutine
		select {
		case <-ticker.C:
			// Iterate through each db, check modtime. If new, reload file
			cityErr := g.open(&g.city, "GeoIP2-City.mmdb", "GeoLite2-City.mmdb")
			if cityErr != nil {
				log.Printf("Failed to update City: %v\n", cityErr)
			}
			countryErr := g.open(&g.country, "GeoIP2-Country.mmdb", "GeoLite2-Country.mmdb")
			if countryErr != nil {
				log.Printf("failed to update Country: %v\n", countryErr)
			}
			asnErr := g.open(&g.asn, "GeoIP2-ASN.mmdb", "GeoLite2-ASN.mmdb")
			if asnErr != nil {
				log.Printf("failed to update ASN: %v\n", asnErr)
			}
		}
	}
}

func (g *GeoIP2) anyActive() bool {
	return g.country.active || g.city.active || g.asn.active
}

// New returns a new GeoIP2 provider
func New(dir string) (g *GeoIP2, err error) {
	g = &GeoIP2{
		dir: dir,
	}
	// This routine MUST load the database files at least once.
	cityErr := g.open(&g.city, "GeoIP2-City.mmdb", "GeoLite2-City.mmdb")
	if cityErr != nil {
		log.Printf("failed to load City DB: %v\n", cityErr)
		err = cityErr
	}
	countryErr := g.open(&g.country, "GeoIP2-Country.mmdb", "GeoLite2-Country.mmdb")
	if countryErr != nil {
		log.Printf("failed to load Country DB: %v\n", countryErr)
		err = countryErr
	}
	asnErr := g.open(&g.asn, "GeoIP2-ASN.mmdb", "GeoLite2-ASN.mmdb")
	if asnErr != nil {
		log.Printf("failed to load ASN DB: %v\n", asnErr)
		err = asnErr
	}
	if !g.anyActive() {
		return nil, err
	}
	go g.watchFiles() // Launch goroutine to load and monitor
	return
}

// HasASN returns if we can do ASN lookups
func (g *GeoIP2) HasASN() (bool, error) {
	return g.asn.active, nil
}

// GetASN returns the ASN for the IP (as a "as123" string) and the netmask
func (g *GeoIP2) GetASN(ip net.IP) (string, int, error) {
	g.asn.l.RLock()
	defer g.asn.l.RUnlock()

	if !g.asn.active {
		return "", 0, fmt.Errorf("ASN db not active")
	}

	c, err := g.asn.db.ASN(ip)
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
	return g.country.active, nil
}

// GetCountry returns the country, continent and netmask for the given IP
func (g *GeoIP2) GetCountry(ip net.IP) (country, continent string, netmask int) {
	// Need a read-lock because return value of Country is a pointer, not copy of the struct/object
	g.country.l.RLock()
	defer g.country.l.RUnlock()

	if !g.country.active {
		return "", "", 0
	}

	c, err := g.country.db.Country(ip)
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

// HasLocation returns if the city database is available to return lat/lon information for an IP
func (g *GeoIP2) HasLocation() (bool, error) {
	return g.city.active, nil
}

// GetLocation returns a geo.Location object for the given IP
func (g *GeoIP2) GetLocation(ip net.IP) (l *geo.Location, err error) {
	// Need a read-lock because return value of City is a pointer, not copy of the struct/object
	g.city.l.RLock()
	defer g.city.l.RUnlock()

	if !g.city.active {
		return nil, fmt.Errorf("city db not active")
	}

	c, err := g.city.db.City(ip)
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
