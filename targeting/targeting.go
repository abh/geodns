package targeting

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/abh/geodns/v3/targeting/geo"
)

type TargetOptions int

const (
	TargetGlobal = 1 << iota
	TargetContinent
	TargetCountry
	TargetRegionGroup
	TargetRegion
	TargetASN
	TargetIP
)

var cidr48Mask net.IPMask

func init() {
	cidr48Mask = net.CIDRMask(48, 128)
}

var g geo.Provider

// Setup sets the global geo provider
func Setup(gn geo.Provider) error {
	g = gn
	return nil
}

// Geo returns the global geo provider
func Geo() geo.Provider {
	return g
}

func (t TargetOptions) getGeoTargets(ip net.IP, hasClosest bool) ([]string, int, *geo.Location) {

	targets := make([]string, 0)

	if t&TargetASN > 0 {
		asn, _, err := g.GetASN(ip)
		if err != nil {
			log.Printf("GetASN error: %s", err)
		}
		if len(asn) > 0 {
			targets = append(targets, asn)
		}
	}

	var country, continent, region, regionGroup string
	var netmask int
	var location *geo.Location

	if t&TargetRegion > 0 || t&TargetRegionGroup > 0 || hasClosest {
		var err error
		location, err = g.GetLocation(ip)
		if location == nil || err != nil {
			return targets, 0, nil
		}
		// log.Printf("Location for '%s' (err: %s): %+v", ip, err, location)
		country = location.Country
		continent = location.Continent
		region = location.Region
		regionGroup = location.RegionGroup
		// continent, regionGroup, region, netmask,

	} else if t&TargetCountry > 0 || t&TargetContinent > 0 {
		country, continent, netmask = g.GetCountry(ip)
	}

	if t&TargetRegion > 0 && len(region) > 0 {
		targets = append(targets, region)
	}
	if t&TargetRegionGroup > 0 && len(regionGroup) > 0 {
		targets = append(targets, regionGroup)
	}

	if t&TargetCountry > 0 && len(country) > 0 {
		targets = append(targets, country)
	}

	if t&TargetContinent > 0 && len(continent) > 0 {
		targets = append(targets, continent)
	}

	return targets, netmask, location
}

func (t TargetOptions) GetTargets(ip net.IP, hasClosest bool) ([]string, int, *geo.Location) {

	targets := make([]string, 0)
	var location *geo.Location
	var netmask int

	if t&TargetIP > 0 {
		ipStr := ip.String()
		targets = append(targets, "["+ipStr+"]")
		ip4 := ip.To4()
		if ip4 != nil {
			if ip4[3] != 0 {
				ip4[3] = 0
				targets = append(targets, "["+ip4.String()+"]")
			}
		} else {
			// v6 address, also target the /48 address
			ip48 := ip.Mask(cidr48Mask)
			targets = append(targets, "["+ip48.String()+"]")
		}
	}

	if g != nil {
		var geotargets []string
		geotargets, netmask, location = t.getGeoTargets(ip, hasClosest)
		targets = append(targets, geotargets...)
	}

	if t&TargetGlobal > 0 {
		targets = append(targets, "@")
	}
	return targets, netmask, location
}

func (t TargetOptions) String() string {
	targets := make([]string, 0)
	if t&TargetGlobal > 0 {
		targets = append(targets, "@")
	}
	if t&TargetContinent > 0 {
		targets = append(targets, "continent")
	}
	if t&TargetCountry > 0 {
		targets = append(targets, "country")
	}
	if t&TargetRegionGroup > 0 {
		targets = append(targets, "regiongroup")
	}
	if t&TargetRegion > 0 {
		targets = append(targets, "region")
	}
	if t&TargetASN > 0 {
		targets = append(targets, "asn")
	}
	if t&TargetIP > 0 {
		targets = append(targets, "ip")
	}
	return strings.Join(targets, " ")
}

func ParseTargets(v string) (tgt TargetOptions, err error) {
	targets := strings.Split(v, " ")
	for _, t := range targets {
		var x TargetOptions
		switch t {
		case "@":
			x = TargetGlobal
		case "country":
			x = TargetCountry
		case "continent":
			x = TargetContinent
		case "regiongroup":
			x = TargetRegionGroup
		case "region":
			x = TargetRegion
		case "asn":
			x = TargetASN
		case "ip":
			x = TargetIP
		default:
			err = fmt.Errorf("unknown targeting option '%s'", t)
		}
		tgt = tgt | x
	}
	return
}
