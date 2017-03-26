package geo

import (
	"math"
	"net"

	"github.com/golang/geo/s2"
)

type Provider interface {
	HasCountry() (bool, error)
	GetCountry(ip net.IP) (country, continent string, netmask int)
	HasASN() (bool, error)
	GetASN(net.IP) (asn string, netmask int, err error)
	HasLocation() (bool, error)
	GetLocation(ip net.IP) (location *Location, err error)
}

const MAX_DISTANCE = 360

type Location struct {
	Country     string
	Continent   string
	RegionGroup string
	Region      string
	Latitude    float64
	Longitude   float64
	Netmask     int
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
