package zones

import (
	"encoding/json"
	"log"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/abh/geodns/v3/applog"
	"github.com/abh/geodns/v3/health"
	"github.com/abh/geodns/v3/targeting"
	"github.com/abh/geodns/v3/targeting/geo"

	dnsv1 "github.com/miekg/dns"
)

type ZoneOptions struct {
	Serial    int
	Ttl       int
	MaxHosts  int
	Contact   string
	Targeting targeting.TargetOptions
	Closest   bool

	// temporary, using this to keep the healthtest code
	// compiling and vaguely included
	healthChecker bool
}

type ZoneLogging struct {
	StatHat    bool
	StatHatAPI string
}

type Record struct {
	RR     dnsv1.RR
	Weight int
	Loc    *geo.Location
	Test   string
}

type Records []*Record

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
	Test     health.HealthTester
}

type LabelMatch struct {
	Label *Label
	Type  uint16
}

type labelmap map[string]*Label

type ZoneMetrics struct {
	LabelStats  *zoneLabelStats
	ClientStats *zoneLabelStats
}

type Zone struct {
	Origin       string
	Labels       labelmap
	LabelCount   int
	Options      ZoneOptions
	Logging      *ZoneLogging
	Metrics      ZoneMetrics
	HasClosest   bool
	HealthStatus health.Status
	healthExport bool

	sync.RWMutex
}

func NewZone(name string) *Zone {
	zone := new(Zone)
	zone.Labels = make(labelmap)
	zone.Origin = name
	zone.LabelCount = dnsv1.CountLabel(zone.Origin)

	// defaults
	zone.Options.Ttl = 120
	zone.Options.MaxHosts = 2
	zone.Options.Contact = "hostmaster." + name
	zone.Options.Targeting = targeting.TargetGlobal + targeting.TargetCountry + targeting.TargetContinent

	return zone
}

func (z *Zone) SetupMetrics(old *Zone) {
	z.Lock()
	defer z.Unlock()

	if old != nil {
		z.Metrics = old.Metrics
	}
	if z.Metrics.LabelStats == nil {
		z.Metrics.LabelStats = NewZoneLabelStats(10000)
	}
	if z.Metrics.ClientStats == nil {
		z.Metrics.ClientStats = NewZoneLabelStats(10000)
	}
}

func (z *Zone) Close() {
	// todo: prune prometheus metrics for the zone ...

	if z.Metrics.LabelStats != nil {
		z.Metrics.LabelStats.Close()
	}
	if z.Metrics.ClientStats != nil {
		z.Metrics.ClientStats.Close()
	}
}

func (l *Label) FirstRR(dnsType uint16) dnsv1.RR {
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

func (z *Zone) SoaRR() dnsv1.RR {
	return z.Labels[""].FirstRR(dnsv1.TypeSOA)
}

func (zone *Zone) AddSOA() {
	zone.addSOA()
}

func (zone *Zone) addSOA() {
	label := zone.Labels[""]

	primaryNs := "ns"

	// log.Println("LABEL", label)

	if label == nil {
		log.Println(zone.Origin, "doesn't have any 'root' records,",
			"you should probably add some NS records")
		label = zone.AddLabel("")
	}

	if record, ok := label.Records[dnsv1.TypeNS]; ok {
		primaryNs = record[0].RR.(*dnsv1.NS).Ns
	}

	ttl := zone.Options.Ttl * 10
	if ttl > 3600 {
		ttl = 3600
	}
	if ttl == 0 {
		ttl = 600
	}

	s := zone.Origin + ". " + strconv.Itoa(ttl) + " IN SOA " +
		primaryNs + " " + zone.Options.Contact + " " +
		strconv.Itoa(zone.Options.Serial) +
		// refresh, retry, expire, minimum are all
		// meaningless with this implementation
		" 5400 5400 1209600 3600"

	// log.Println("SOA: ", s)

	rr, err := dnsv1.NewRR(s)
	if err != nil {
		log.Println("SOA Error", err)
		panic("Could not setup SOA")
	}

	record := Record{RR: rr}

	label.Records[dnsv1.TypeSOA] = make([]*Record, 1)
	label.Records[dnsv1.TypeSOA][0] = &record
}

func (z *Zone) findFirstLabel(s string, targets []string, qts []uint16) *LabelMatch {
	matches := z.FindLabels(s, targets, qts)
	if len(matches) == 0 {
		return nil
	}
	return &matches[0]
}

// Find label "s" in country "cc" falling back to the appropriate
// continent and the global label name as needed. Looks for the
// first available qType at each targeting level. Returns a list of
// LabelMatch for potential labels that might satisfy the query.
// "MF" records are treated as aliases. The API returns all the
// matches the targeting will allow so health check filtering won't
// filter out the "best" results leaving no others.
func (z *Zone) FindLabels(s string, targets []string, qts []uint16) []LabelMatch {
	matches := make([]LabelMatch, 0)

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
				case dnsv1.TypeANY:
					// short-circuit mostly to avoid subtle bugs later
					// to be correct we should run through all the selectors and
					// pick types not already picked
					matches = append(matches, LabelMatch{z.Labels[s], qtype})
					continue
				case dnsv1.TypeMF:
					if label.Records[dnsv1.TypeMF] != nil {

						// don't follow NS and SOA records for aliases
						aliasQts := slices.DeleteFunc(qts, func(q uint16) bool {
							if slices.Contains(
								[]uint16{dnsv1.TypeNS, dnsv1.TypeSOA},
								q) {
								return true
							}
							return false
						})

						name = label.FirstRR(dnsv1.TypeMF).(*dnsv1.MF).Mf
						// TODO: need to avoid loops here somehow
						aliases := z.FindLabels(name, targets, aliasQts)
						matches = append(matches, aliases...)
						continue
					}
				default:
					// return the label if it has the right record
					if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
						matches = append(matches, LabelMatch{label, qtype})
						continue
					}
				}
			}
		}
	}

	if len(matches) == 0 {
		// this is to make sure we return 'noerror' instead of 'nxdomain' when
		// appropriate.
		if label, ok := z.Labels[s]; ok {
			matches = append(matches, LabelMatch{label, 0})
		}
	}

	return matches
}

// Find the locations of all the A and AAAA records within a zone. If we were
// being really clever here we could use LOC records too. But for the time
// being we'll just use GeoIP.
func (z *Zone) SetLocations() {
	geo := targeting.Geo()
	qtypes := []uint16{dnsv1.TypeA, dnsv1.TypeAAAA}
	for _, label := range z.Labels {
		if label.Closest {
			for _, qtype := range qtypes {
				if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
					for i := range label.Records[qtype] {
						label.Records[qtype][i].Loc = nil
						rr := label.Records[qtype][i].RR
						var ip *net.IP
						switch rr.(type) {
						case *dnsv1.A:
							ip = &rr.(*dnsv1.A).A
						case *dnsv1.AAAA:
							ip = &rr.(*dnsv1.AAAA).AAAA
						default:
							log.Printf("Can't lookup location of type %T", rr)
						}
						if ip != nil {
							location, err := geo.GetLocation(*ip)
							if err != nil {
								// log.Printf("Could not get location for '%s': %s", ip.String(), err)
								continue
							}
							label.Records[qtype][i].Loc = location
						}
					}
				}
			}
		}
	}
}

func (z *Zone) addHealthReference(l *Label, data interface{}) {
	// First safely get rid of any old test. As label tests
	// should never run this should never be executed
	// if l.Test != nil {
	// 	l.Test.Stop()
	// 	l.Test = nil
	// }

	if data == nil {
		return
	}

	if i, ok := data.(map[string]interface{}); ok {
		tester, err := health.NewReferenceFromMap(i)
		if err != nil {
			applog.Printf("Could not setup reference to health check: %s", err)
			return
		}
		l.Test = tester
	}
}

func (z *Zone) setupHealthTests() {
	for _, label := range z.Labels {
		if label.Test == nil {
			// log.Printf("label.Test for '%s' == nil", label.Label)
			continue
		}

		// todo: document which record types are processed
		// or process all ...
		for _, rrs := range label.Records {
			for _, rec := range rrs {
				if len(rec.Test) > 0 {
					continue
				}
				var t string
				switch rrt := rec.RR.(type) {
				case *dnsv1.A:
					t = rrt.A.String()
				case *dnsv1.AAAA:
					t = rrt.AAAA.String()
				case *dnsv1.MX:
					t = rrt.Mx
				default:
					continue
				}
				rec.Test = t
			}
		}
	}
}

// func (z *Zone) StartStopHealthTests(start bool, oldZone *Zone) {}
// 	applog.Printf("Start/stop health checks on zone %s start=%v", z.Origin, start)
// for labelName, label := range z.Labels {
// 		for _, qtype := range health.Qtypes {
// 			if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
// 				for i := range label.Records[qtype] {
// 					rr := label.Records[qtype][i].RR
// 					var ip net.IP
// 					switch rrt := rr.(type) {
// 					case *dnsv1.A:
// 						ip = rrt.A
// 					case *dnsv1.AAAA:
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
// 											case *dnsv1.A:
// 												oip = orrt.A
// 											case *dnsv1.AAAA:
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

func (z *Zone) HealthRR(label string, baseLabel string) []dnsv1.RR {
	h := dnsv1.RR_Header{Ttl: 1, Class: dnsv1.ClassINET, Rrtype: dnsv1.TypeTXT}
	h.Name = label

	healthstatus := make(map[string]map[string]bool)

	// if l, ok := z.Labels[baseLabel]; ok {
	// 	for qt, records := range l.Records {
	// 		if qts, ok := dnsv1.TypeToString[qt]; ok {
	// 			hmap := make(map[string]bool)
	// 			for _, record := range records {
	// 				if record.Test != nil {
	// 					hmap[(*record.Test).IP().String()] = health.TestRunner.IsHealthy(record.Test)
	// 				}
	// 			}
	// 			healthstatus[qts] = hmap
	// 		}
	// 	}
	// }

	js, _ := json.Marshal(healthstatus)

	return []dnsv1.RR{&dnsv1.TXT{Hdr: h, Txt: []string{string(js)}}}
}
