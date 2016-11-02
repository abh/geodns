package main

import (
	"sort"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
	glob "github.com/ryanuber/go-glob"
)

type ZoneOptions struct {
	Serial    int
	Ttl       int
	MaxHosts  int
	Contact   string
	Targeting TargetOptions
}

type ZoneLogging struct {
	StatHat    bool
	StatHatAPI string
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
type globLabels []*Label

func (l globLabels) Len() int      { return len(l) }
func (l globLabels) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l globLabels) Less(i, j int) bool {
	return len(strings.Replace(l[i].Label, "*", "", -1)) < len(strings.Replace(l[j].Label, "*", "", -1))
}

type ZoneMetrics struct {
	Queries     metrics.Meter
	EdnsQueries metrics.Meter
	Registry    metrics.Registry
	LabelStats  *zoneLabelStats
	ClientStats *zoneLabelStats
}

type Zone struct {
	Origin     string
	GlobLabels globLabels
	Labels     labels
	LabelCount int
	Options    ZoneOptions
	Logging    *ZoneLogging
	Metrics    ZoneMetrics

	sync.RWMutex
}

type qTypes []uint16

func NewZone(name string) *Zone {
	zone := new(Zone)
	zone.Labels = make(labels)
	zone.Origin = name
	zone.LabelCount = dns.CountLabel(zone.Origin)

	// defaults
	zone.Options.Ttl = 120
	zone.Options.MaxHosts = 2
	zone.Options.Contact = "hostmaster." + name
	zone.Options.Targeting = TargetGlobal + TargetCountry + TargetContinent

	return zone
}

func (z *Zone) SetupMetrics(old *Zone) {
	z.Lock()
	defer z.Unlock()

	if old != nil {
		z.Metrics = old.Metrics
	}
	if z.Metrics.Registry == nil {
		z.Metrics.Registry = metrics.NewRegistry()
	}
	if z.Metrics.Queries == nil {
		z.Metrics.Queries = metrics.NewMeter()
		z.Metrics.Registry.Register("queries", z.Metrics.Queries)
	}
	if z.Metrics.EdnsQueries == nil {
		z.Metrics.EdnsQueries = metrics.NewMeter()
		z.Metrics.Registry.Register("queries-edns", z.Metrics.EdnsQueries)
	}
	if z.Metrics.LabelStats == nil {
		z.Metrics.LabelStats = NewZoneLabelStats(10000)
	}
	if z.Metrics.ClientStats == nil {
		z.Metrics.ClientStats = NewZoneLabelStats(10000)
	}
}

func (z *Zone) Close() {
	z.Metrics.Registry.UnregisterAll()
	if z.Metrics.LabelStats != nil {
		z.Metrics.LabelStats.Close()
	}
	if z.Metrics.ClientStats != nil {
		z.Metrics.ClientStats.Close()
	}
}

func (l *Label) firstRR(dnsType uint16) dns.RR {
	return l.Records[dnsType][0].RR
}

func (z *Zone) AddLabel(k string) *Label {
	k = strings.ToLower(k)
	label := &Label{
		Label:    k,
		Ttl:      z.Options.Ttl,
		MaxHosts: z.Options.MaxHosts,
		Records:  make(map[uint16]Records),
		Weight:   make(map[uint16]int),
	}

	if !strings.Contains(k, "*") {
		z.Labels[k] = label
	} else {
		z.GlobLabels = append(z.GlobLabels, label)
		sort.Sort(sort.Reverse(z.GlobLabels))
	}

	return label
}

func (z *Zone) SoaRR() dns.RR {
	return z.Labels[""].firstRR(dns.TypeSOA)
}

// Find label "s" in country "cc" falling back to the appropriate
// continent and the global label name as needed. Looks for the
// first available qType at each targeting level. Return a Label
// and the qtype that was "found"
func (z *Zone) findLabels(s string, targets []string, qts qTypes) (*Label, uint16) {
	for _, target := range targets {
		var name string

		switch target {
		case "@":
			name = s
		default:
			if len(s) > 0 {
				name = s + "." + target
			} else {
				name = target
			}
		}

		if label, ok := z.Labels[name]; ok {
			var name string
			for _, qtype := range qts {
				switch qtype {
				case dns.TypeANY:
					// short-circuit mostly to avoid subtle bugs later
					// to be correct we should run through all the selectors and
					// pick types not already picked
					return z.Labels[s], qtype
				case dns.TypeMF:
					if label.Records[dns.TypeMF] != nil {
						name = label.firstRR(dns.TypeMF).(*dns.MF).Mf
						// TODO: need to avoid loops here somehow
						return z.findLabels(name, targets, qts)
					}
				default:
					// return the label if it has the right record
					if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
						return label, qtype
					}
				}
			}
		}
	}
	// check against each glob label
	var found bool
	for n, label := range z.GlobLabels {
		if found {
			break
		}
		for _, target := range targets { // iterate again
			var name string

			switch target {
			case "@":
				name = s
			default:
				if len(s) > 0 {
					name = s + "." + target
				} else {
					name = target
				}
			}
			if _, ok := z.Labels[s]; !ok && glob.Glob(z.GlobLabels[n].Label, name) {
				found = true
				for _, qtype := range qts {
					switch qtype {
					case dns.TypeANY:
						// short-circuit mostly to avoid subtle bugs later
						// to be correct we should run through all the selectors and
						// pick types not already picked
						return z.GlobLabels[n], qtype
					case dns.TypeMF:
						if label.Records[dns.TypeMF] != nil {
							name = label.firstRR(dns.TypeMF).(*dns.MF).Mf
							// TODO: need to avoid loops here somehow
							return z.findLabels(name, targets, qts)
						}
					default:
						// return the label if it has the right record
						if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
							return label, qtype
						}
					}
				}

			}

		}
	} // glob
	if found {
		// we need to return NOERROR if there is at least one label
		// otherwise geodns will return NXDOMAIN, which will be cached by other dns servers
		// so they will return nothing for any consequent query
		return new(Label), 0
	}

	return z.Labels[s], 0
}
