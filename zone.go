package main

import (
	"strings"
	"sync"

	"github.com/abh/geodns/applog"
	"github.com/abh/geodns/health"

	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
)

type ZoneOptions struct {
	Serial    int
	Ttl       int
	MaxHosts  int
	Contact   string
	Targeting TargetOptions
	Closest   bool
}

type ZoneLogging struct {
	StatHat    bool
	StatHatAPI string
}

type Record struct {
	RR     dns.RR
	Weight int
	Loc    *Location
	Test   *health.HealthTest
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
	Closest  bool
	Test     *health.HealthTest
}

type labels map[string]*Label

type ZoneMetrics struct {
	Queries     metrics.Meter
	EdnsQueries metrics.Meter
	Registry    metrics.Registry
	LabelStats  *zoneLabelStats
	ClientStats *zoneLabelStats
}

type Zone struct {
	Origin     string
	Labels     labels
	LabelCount int
	Options    ZoneOptions
	Logging    *ZoneLogging
	Metrics    ZoneMetrics
	HasClosest bool
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
	z.StartStopHealthChecks(false, nil)
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
	z.Labels[k] = new(Label)
	label := z.Labels[k]
	label.Label = k
	label.Ttl = 0 // replaced later
	label.MaxHosts = z.Options.MaxHosts
	label.Closest = z.Options.Closest

	label.Records = make(map[uint16]Records)
	label.Weight = make(map[uint16]int)

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

	return z.Labels[s], 0
}

// Find the locations of all the A records within a zone. If we were being really clever
// here we could use LOC records too. But for the time being we'll just use GeoIP
func (z *Zone) SetLocations() {
	qtypes := []uint16{dns.TypeA}
	for _, label := range z.Labels {
		if label.Closest {
			for _, qtype := range qtypes {
				if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
					for i := range label.Records[qtype] {
						label.Records[qtype][i].Loc = nil
						rr := label.Records[qtype][i].RR
						if a, ok := rr.(*dns.A); ok {
							ip := a.A
							_, _, _, _, _, location := geoIP.GetCountryRegion(ip)
							label.Records[qtype][i].Loc = location
						}
					}
				}
			}
		}
	}
}

func (z *Zone) newHealthTest(l *Label, data interface{}) {
	// First safely get rid of any old test. As label tests
	// should never run this should never be executed
	if l.Test != nil {
		l.Test.Stop()
		l.Test = nil
	}

	if data == nil {
		return
	}

	if i, ok := data.(map[string]interface{}); ok {
		tester, err := health.NewFromMap(i)
		if err != nil {
			applog.Printf("Could not configure health check: %s", err)
			return
		}
		l.Test = tester

	}
}

func (z *Zone) StartStopHealthChecks(start bool, oldZone *Zone) {
	// 	applog.Printf("Start/stop health checks on zone %s start=%v", z.Origin, start)
	// 	for labelName, label := range z.Labels {
	// 		for _, qtype := range health.Qtypes {
	// 			if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
	// 				for i := range label.Records[qtype] {
	// 					rr := label.Records[qtype][i].RR
	// 					var ip net.IP
	// 					switch rrt := rr.(type) {
	// 					case *dns.A:
	// 						ip = rrt.A
	// 					case *dns.AAAA:
	// 						ip = rrt.AAAA
	// 					default:
	// 						continue
	// 					}

	// 					var test *health.HealthTest
	// 					ref := fmt.Sprintf("%s/%s/%d/%d", z.Origin, labelName, qtype, i)
	// 					if start {
	// 						if test = label.Records[qtype][i].Test; test != nil {
	// 							// stop any old test
	// 							health.TestRunner.removeTest(test, ref)
	// 						} else {
	// 							if ltest := label.Test; ltest != nil {
	// 								test = ltest.copy(ip)
	// 								label.Records[qtype][i].Test = test
	// 							}
	// 						}
	// 						if test != nil {
	// 							test.ipAddress = ip
	// 							// if we are given an oldzone, let's see if we can find the old RR and
	// 							// copy over the initial health state, rather than use the initial health
	// 							// state provided from the label. This helps to stop health state bouncing
	// 							// when a zone file is reloaded for a purposes unrelated to the RR
	// 							if oldZone != nil {
	// 								oLabel, ok := oldZone.Labels[labelName]
	// 								if ok {
	// 									if oLabel.Test != nil {
	// 										for i := range oLabel.Records[qtype] {
	// 											oRecord := oLabel.Records[qtype][i]
	// 											var oip net.IP
	// 											switch orrt := oRecord.RR.(type) {
	// 											case *dns.A:
	// 												oip = orrt.A
	// 											case *dns.AAAA:
	// 												oip = orrt.AAAA
	// 											default:
	// 												continue
	// 											}
	// 											if oip.Equal(ip) {
	// 												if oRecord.Test != nil {
	// 													h := oRecord.Test.IsHealthy()
	// 													applog.Printf("Carrying over previous health state for %s: %v", oRecord.Test.ipAddress, h)
	// 													// we know the test is stopped (as we haven't started it) so we can write
	// 													// without the mutex and avoid a misleading log message
	// 													test.healthy = h
	// 												}
	// 												break
	// 											}
	// 										}
	// 									}
	// 								}
	// 							}
	// 							health.TestRunner.addTest(test, ref)
	// 						}
	// 					} else {
	// 						if test = label.Records[qtype][i].Test; test != nil {
	// 							health.TestRunner.removeTest(test, ref)
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
}
