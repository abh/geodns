package main

import (
	"github.com/miekg/dns"
)

type Options struct {
	Serial int
	Ttl    int
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
		return z.Labels[s]
	}

	selectors := []string{}

	if len(cc) > 0 {
		if len(s) > 0 {
			cc = s + "." + cc
		}
		selectors = append(selectors, cc)
	}
	// TODO(ask) Add continent, see https://github.com/abh/geodns/issues/1
	selectors = append(selectors, s)

	for _, name := range selectors {
		if label, ok := z.Labels[name]; ok {
			// return the label if it has the right records
			// TODO(ask) Should this also look for CNAME or aliases?
			if label.Records[qtype] != nil {
				return label
			}
		}
	}
	return z.Labels[s]
}
