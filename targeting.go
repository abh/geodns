package main

import (
	"fmt"
	"net"
	"strings"
)

type TargetOptions int

const (
	TargetGlobal = 1 << iota
	TargetContinent
	TargetCountry
	TargetRegionGroup
	TargetRegion
)

func (t TargetOptions) GetTargets(ip net.IP) ([]string, int) {

	targets := make([]string, 0)

	var country, continent string
	var netmask int

	switch {
	case t >= TargetRegionGroup:
		var region, regionGroup string
		country, continent, regionGroup, region, netmask = geoIP.GetCountryRegion(ip)
		if t&TargetRegion > 0 && len(region) > 0 {
			targets = append(targets, region)
		}
		if t&TargetRegionGroup > 0 && len(regionGroup) > 0 {
			targets = append(targets, regionGroup)
		}

	case t >= TargetContinent:
		country, continent, netmask = geoIP.GetCountry(ip)
	}

	if len(country) > 0 {
		if t&TargetCountry > 0 {
			targets = append(targets, country)
		}
		if t&TargetContinent > 0 && len(continent) > 0 {
			targets = append(targets, continent)
		}
	}

	if t&TargetGlobal > 0 {
		targets = append(targets, "@")
	}
	return targets, netmask
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
	return strings.Join(targets, " ")
}

func parseTargets(v string) (tgt TargetOptions, err error) {
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
		default:
			err = fmt.Errorf("Unknown targeting option '%s'", t)
		}
		tgt = tgt | x
	}
	return
}
