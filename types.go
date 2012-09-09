package main

import (
	"geodns/countries"
	"github.com/miekg/dns"
)

type Options struct {
	Serial   int
	Ttl      int
	MaxHosts int
}

type Record struct {
	RR     dns.RR
	Weight int
}

type Records []Record

func (s Records) Len() int      { return len(s) }
func (s Records) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type RecordsByWeight struct{ Records }

func (s RecordsByWeight) Less(i, j int) bool { return s.Records[i].Weight > s.Records[j].Weight }

type Label struct {
	Label    string
	MaxHosts int
	Ttl      int
	Records  map[uint16]Records
	Weight   map[uint16]int
}

type labels map[string]*Label

type Zones map[string]*Zone

type Zone struct {
	Origin    string
	Labels    labels
	LenLabels int
	Options   Options
}

func (l *Label) firstRR(dnsType uint16) dns.RR {
	return l.Records[dnsType][0].RR
}

func (z *Zone) SoaRR() dns.RR {
	return z.Labels[""].firstRR(dns.TypeSOA)
}

func (z *Zone) findLabels(s, cc string, qtype uint16) *Label {

	if qtype == dns.TypeANY {
		// short-circuit mostly to avoid subtle bugs later
		// to be correct we should run through all the selectors and
		// pick types not already picked
		return z.Labels[s]
	}

	selectors := []string{}

	if len(cc) > 0 {
		continent := countries.CountryContinent[cc]
		if len(s) > 0 {
			cc = s + "." + cc
			if len(continent) > 0 {
				continent = s + "." + continent
			}
		}
		selectors = append(selectors, cc, continent)
	}
	selectors = append(selectors, s)

	for _, name := range selectors {
		if label, ok := z.Labels[name]; ok {

			// look for aliases
			if label.Records[dns.TypeMF] != nil {
				name = label.firstRR(dns.TypeMF).(*dns.RR_MF).Mf
				// BUG(ask) - restructure this so it supports chains of aliases
				label, ok = z.Labels[name]
				if label == nil {
					continue
				}
			}

			// return the label if it has the right records
			// TODO(ask) Should this also look for CNAME records?
			if label.Records[qtype] != nil {
				return label
			}
		}
	}
	return z.Labels[s]
}
