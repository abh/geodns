package geo

import (
	"log"
	"net"
	"time"

	"github.com/abh/geoip"
)

type Geotargeter interface {
	SetDirectory(string)

	GetCountry(net.IP) (country, continent string, netmask int)
	GetCountryRegion(ip net.IP) (country, continent, regionGroup, region string, netmask int)
	GetASN(ip net.IP) (asn string, netmask int)

	SetupCity() error
	SetupCountry() error
	SetupASN() error
}

type GeoIP struct {
	DisableV6 bool

	v4 geodbs
	v6 geodbs
}

func New() Geotargeter {
	g := new(GeoIP)
	return g
}

func (g *GeoIP) GetCountry(ip net.IP) (country, continent string, netmask int) {
	db := g.ipdb(ip)
	if db == nil {
		return
	}
	return db.GetCountry(ip)
}

func (g *GeoIP) GetCountryRegion(ip net.IP) (country, continent, regionGroup, region string, netmask int) {
	db := g.ipdb(ip)
	if db == nil {
		return
	}
	return db.GetCountryRegion(ip)
}

func (g *GeoIP) GetASN(ip net.IP) (asn string, netmask int) {
	db := g.ipdb(ip)
	if db == nil {
		return
	}
	return db.GetASN(ip)
}

func (g *GeoIP) ipdb(ip net.IP) *geodbs {
	switch isv6(ip) {
	case false:
		return &g.v4
	case true:
		return &g.v6
	}
	panic("impossible")
}

func isv6(ip net.IP) bool {
	ip4 := ip.To4()
	rv := ip4 == nil
	return rv
}

func (g *GeoIP) SetDirectory(dir string) {
	if len(dir) > 0 {
		geoip.SetCustomDirectory(dir)
	}
}

func loadDB(db *geodb, geotype int) error {
	if db.loaded {
		return nil
	}

	gi, err := geoip.OpenType(geotype)
	if gi == nil || err != nil {
		log.Printf("Could not open GeoIP database (%d): %s\n", geotype, err)
		return err
	}
	db.lastLoad = time.Now()
	db.loaded = true
	db.db = gi

	return nil
}

func (g *GeoIP) SetupCity() error {
	var err, err6 error

	if g.v4.city.loaded == false {
		err = loadDB(&g.v4.city, geoip.GEOIP_CITY_EDITION_REV1)
	}

	if g.v6.city.loaded == false && g.DisableV6 == false {
		err6 = loadDB(&g.v6.city, geoip.GEOIP_CITY_EDITION_REV1_V6)
	}

	if err != nil {
		return err
	}

	return err6
}

func (g *GeoIP) SetupCountry() error {
	var err, err6 error

	if g.v4.country.loaded == false {
		err = loadDB(&g.v4.country, geoip.GEOIP_COUNTRY_EDITION)
	}

	if g.v6.country.loaded == false && g.DisableV6 == false {
		err6 = loadDB(&g.v6.country, geoip.GEOIP_COUNTRY_EDITION_V6)
	}

	if err != nil {
		return err
	}

	return err6
}

func (g *GeoIP) SetupASN() error {
	var err, err6 error

	if g.v4.asn.loaded == false {
		err = loadDB(&g.v4.asn, geoip.GEOIP_ASNUM_EDITION)
	}

	if g.v6.asn.loaded == false && g.DisableV6 == false {
		err6 = loadDB(&g.v6.asn, geoip.GEOIP_ASNUM_EDITION_V6)
	}

	if err != nil {
		return err
	}

	return err6
}
