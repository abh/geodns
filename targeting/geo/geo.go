package geo

import (
	"math"
	"net"

	"github.com/golang/geo/s2"
)

// Provider is the interface for geoip providers
type Provider interface {
	HasCountry() (bool, error)
	GetCountry(ip net.IP) (country, continent string, netmask int)
	HasASN() (bool, error)
	GetASN(net.IP) (asn string, netmask int, err error)
	HasLocation() (bool, error)
	GetLocation(ip net.IP) (location *Location, err error)
}

// MaxDistance is the distance returned if Distance() is
// called with a nil location
const MaxDistance = 360

// Location is the struct the GeoIP provider packages use to
// return location details for an IP.
type Location struct {
	Country     string
	Continent   string
	RegionGroup string
	Region      string
	Latitude    float64
	Longitude   float64
	Netmask     int
}

// MaxDistance() returns the MaxDistance constant
func (l *Location) MaxDistance() float64 {
	return MaxDistance
}

// Distance returns the distance between the two locations
func (l *Location) Distance(to *Location) float64 {
	if to == nil {
		return MaxDistance
	}
	ll1 := s2.LatLngFromDegrees(l.Latitude, l.Longitude)
	ll2 := s2.LatLngFromDegrees(to.Latitude, to.Longitude)
	angle := ll1.Distance(ll2)
	return math.Abs(angle.Degrees())
}
