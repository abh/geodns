package zones

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/abh/geodns/targeting"
	"github.com/abh/geodns/typeutil"

	"github.com/abh/errorutil"
	"github.com/miekg/dns"
)

// ZoneList maps domain names to zone data
type ZoneList map[string]*Zone

func (zone *Zone) ReadZoneFile(fileName string) (zerr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("reading %s failed: %s", zone.Origin, r)
			debug.PrintStack()
			zerr = fmt.Errorf("reading %s failed: %s", zone.Origin, r)
		}
	}()

	fh, err := os.Open(fileName)
	if err != nil {
		log.Printf("Could not read '%s': %s", fileName, err)
		panic(err)
	}

	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("Could not stat '%s': %s", fileName, err)
	} else {
		zone.Options.Serial = int(fileInfo.ModTime().Unix())
	}

	var objmap map[string]interface{}
	decoder := json.NewDecoder(fh)
	if err = decoder.Decode(&objmap); err != nil {
		extra := ""
		if serr, ok := err.(*json.SyntaxError); ok {
			if _, serr := fh.Seek(0, os.SEEK_SET); serr != nil {
				log.Fatalf("seek error: %v", serr)
			}
			line, col, highlight := errorutil.HighlightBytePosition(fh, serr.Offset)
			extra = fmt.Sprintf(":\nError at line %d, column %d (file offset %d):\n%s",
				line, col, serr.Offset, highlight)
		}
		return fmt.Errorf("error parsing JSON object in config file %s%s\n%v",
			fh.Name(), extra, err)
	}

	//log.Println(objmap)

	var data map[string]interface{}

	for k, v := range objmap {
		//log.Printf("k: %s v: %#v, T: %T\n", k, v, v)

		switch k {
		case "ttl":
			zone.Options.Ttl = typeutil.ToInt(v)
		case "serial":
			zone.Options.Serial = typeutil.ToInt(v)
		case "contact":
			zone.Options.Contact = v.(string)
		case "max_hosts":
			zone.Options.MaxHosts = typeutil.ToInt(v)
		case "closest":
			zone.Options.Closest = v.(bool)
			if zone.Options.Closest {
				zone.HasClosest = true
			}
		case "targeting":
			zone.Options.Targeting, err = targeting.ParseTargets(v.(string))
			if err != nil {
				return fmt.Errorf("parsing targeting '%s': %s", v, err)
			}

		case "logging":
			{
				logging := new(ZoneLogging)
				for logger, v := range v.(map[string]interface{}) {
					switch logger {
					case "stathat":
						logging.StatHat = typeutil.ToBool(v)
					case "stathat_api":
						logging.StatHatAPI = typeutil.ToString(v)
						logging.StatHat = true
					default:
						log.Println("Unknown logger option", logger)
					}
				}
				zone.Logging = logging
				// log.Printf("logging options: %#v", logging)
			}
			continue

		case "data":
			data = v.(map[string]interface{})
		}
	}

	setupZoneData(data, zone)

	//log.Printf("ZO T: %T %s\n", Zones["0.us"], Zones["0.us"])

	//log.Println("IP", string(Zone.Regions["0.us"].IPv4[0].ip))

	switch {
	case zone.Options.Targeting >= targeting.TargetRegionGroup || zone.HasClosest:
		targeting.GeoIP().SetupGeoIPCity()
	case zone.Options.Targeting >= targeting.TargetContinent:
		targeting.GeoIP().SetupGeoIPCountry()
	}
	if zone.Options.Targeting&targeting.TargetASN > 0 {
		targeting.GeoIP().SetupGeoIPASN()
	}

	if zone.HasClosest {
		zone.SetLocations()
	}

	return nil
}

func setupZoneData(data map[string]interface{}, zone *Zone) {
	recordTypes := map[string]uint16{
		"a":     dns.TypeA,
		"aaaa":  dns.TypeAAAA,
		"alias": dns.TypeMF,
		"cname": dns.TypeCNAME,
		"mx":    dns.TypeMX,
		"ns":    dns.TypeNS,
		"txt":   dns.TypeTXT,
		"spf":   dns.TypeSPF,
		"srv":   dns.TypeSRV,
		"ptr":   dns.TypePTR,
	}

	for dk, dv_inter := range data {
		dv := dv_inter.(map[string]interface{})

		//log.Printf("K %s V %s TYPE-V %T\n", dk, dv, dv)

		label := zone.AddLabel(dk)

		for rType, rdata := range dv {
			switch rType {
			case "max_hosts":
				label.MaxHosts = typeutil.ToInt(rdata)
				continue
			case "closest":
				label.Closest = rdata.(bool)
				if label.Closest {
					zone.HasClosest = true
				}
				continue
			case "ttl":
				label.Ttl = typeutil.ToInt(rdata)
				continue
			case "health":
				zone.addHealthReference(label, rdata)
				continue
			}

			dnsType, ok := recordTypes[rType]
			if !ok {
				log.Printf("Unsupported record type '%s'\n", rType)
				continue
			}

			if rdata == nil {
				//log.Printf("No %s records for label %s\n", rType, dk)
				continue
			}

			//log.Printf("rdata %s TYPE-R %T\n", rdata, rdata)

			records := make(map[string][]interface{})

			switch rdata.(type) {
			case map[string]interface{}:
				// Handle NS map syntax, map[ns2.example.net:<nil> ns1.example.net:<nil>]
				tmp := make([]interface{}, 0)
				for rdataK, rdataV := range rdata.(map[string]interface{}) {
					if rdataV == nil {
						rdataV = ""
					}
					tmp = append(tmp, []string{rdataK, rdataV.(string)})
				}
				records[rType] = tmp
			case string:
				// CNAME and alias
				tmp := make([]interface{}, 1)
				tmp[0] = rdata.(string)
				records[rType] = tmp
			default:
				records[rType] = rdata.([]interface{})
			}

			//log.Printf("RECORDS %s TYPE-REC %T\n", Records, Records)

			label.Records[dnsType] = make(Records, len(records[rType]))

			for i := 0; i < len(records[rType]); i++ {
				//log.Printf("RT %T %#v\n", records[rType][i], records[rType][i])

				record := new(Record)

				var h dns.RR_Header
				h.Class = dns.ClassINET
				h.Rrtype = dnsType

				{
					// allow for individual health test name overrides
					if rec, ok := records[rType][i].(map[string]interface{}); ok {
						if h, ok := rec["health"].(string); ok {
							record.Test = h
						}

					}
				}

				switch len(label.Label) {
				case 0:
					h.Name = zone.Origin + "."
				default:
					h.Name = label.Label + "." + zone.Origin + "."
				}

				switch dnsType {
				case dns.TypeA, dns.TypeAAAA, dns.TypePTR:
					// todo: check interface type
					str, weight := getStringWeight(records[rType][i].([]interface{}))
					ip := str
					record.Weight = weight

					switch dnsType {
					case dns.TypePTR:
						record.RR = &dns.PTR{Hdr: h, Ptr: ip}
						break
					case dns.TypeA:
						if x := net.ParseIP(ip); x != nil {
							record.RR = &dns.A{Hdr: h, A: x}
							break
						}
						panic(fmt.Errorf("Bad A record %s for %s", ip, dk))
					case dns.TypeAAAA:
						if x := net.ParseIP(ip); x != nil {
							record.RR = &dns.AAAA{Hdr: h, AAAA: x}
							break
						}
						panic(fmt.Errorf("Bad AAAA record %s for %s", ip, dk))
					}

				case dns.TypeMX:
					rec := records[rType][i].(map[string]interface{})
					pref := uint16(0)
					mx := rec["mx"].(string)
					if !strings.HasSuffix(mx, ".") {
						mx = mx + "."
					}
					if rec["weight"] != nil {
						record.Weight = typeutil.ToInt(rec["weight"])
					}
					if rec["preference"] != nil {
						pref = uint16(typeutil.ToInt(rec["preference"]))
					}
					record.RR = &dns.MX{
						Hdr:        h,
						Mx:         mx,
						Preference: pref}

				case dns.TypeSRV:
					rec := records[rType][i].(map[string]interface{})
					priority := uint16(0)
					srv_weight := uint16(0)
					port := uint16(0)
					target := rec["target"].(string)

					if !dns.IsFqdn(target) {
						target = target + "." + zone.Origin
					}

					if rec["srv_weight"] != nil {
						srv_weight = uint16(typeutil.ToInt(rec["srv_weight"]))
					}
					if rec["port"] != nil {
						port = uint16(typeutil.ToInt(rec["port"]))
					}
					if rec["priority"] != nil {
						priority = uint16(typeutil.ToInt(rec["priority"]))
					}
					record.RR = &dns.SRV{
						Hdr:      h,
						Priority: priority,
						Weight:   srv_weight,
						Port:     port,
						Target:   target}

				case dns.TypeCNAME:
					rec := records[rType][i]
					var target string
					var weight int
					switch rec.(type) {
					case string:
						target = rec.(string)
					case []interface{}:
						target, weight = getStringWeight(rec.([]interface{}))
					case map[string]interface{}:
						r := rec.(map[string]interface{})

						if t, ok := r["cname"]; ok {
							target = typeutil.ToString(t)
						}

						if w, ok := r["weight"]; ok {
							weight = typeutil.ToInt(w)
						}

						if h, ok := r["health"]; ok {
							record.Test = typeutil.ToString(h)
						}
					}
					if !dns.IsFqdn(target) {
						target = target + "." + zone.Origin
					}
					record.Weight = weight
					record.RR = &dns.CNAME{Hdr: h, Target: dns.Fqdn(target)}

				case dns.TypeMF:
					rec := records[rType][i]
					// MF records (how we store aliases) are not FQDNs
					record.RR = &dns.MF{Hdr: h, Mf: rec.(string)}

				case dns.TypeNS:
					rec := records[rType][i]

					var ns string

					switch rec.(type) {
					case string:
						ns = rec.(string)
					case []string:
						recl := rec.([]string)
						ns = recl[0]
						if len(recl[1]) > 0 {
							log.Println("NS records with names syntax not supported")
						}
					default:
						log.Printf("Data: %T %#v\n", rec, rec)
						panic("Unrecognized NS format/syntax")
					}

					rr := &dns.NS{Hdr: h, Ns: dns.Fqdn(ns)}

					record.RR = rr

				case dns.TypeTXT:
					rec := records[rType][i]

					var txt string

					switch rec.(type) {
					case string:
						txt = rec.(string)
					case map[string]interface{}:

						recmap := rec.(map[string]interface{})

						if weight, ok := recmap["weight"]; ok {
							record.Weight = typeutil.ToInt(weight)
						}
						if t, ok := recmap["txt"]; ok {
							txt = t.(string)
						}
					}
					if len(txt) > 0 {
						rr := &dns.TXT{Hdr: h, Txt: []string{txt}}
						record.RR = rr
					} else {
						log.Printf("Zero length txt record for '%s' in '%s'\n", label.Label, zone.Origin)
						continue
					}
					// Initial SPF support added here, cribbed from the TypeTXT case definition - SPF records should be handled identically

				case dns.TypeSPF:
					rec := records[rType][i]

					var spf string

					switch rec.(type) {
					case string:
						spf = rec.(string)
					case map[string]interface{}:

						recmap := rec.(map[string]interface{})

						if weight, ok := recmap["weight"]; ok {
							record.Weight = typeutil.ToInt(weight)
						}
						if t, ok := recmap["spf"]; ok {
							spf = t.(string)
						}
					}
					if len(spf) > 0 {
						rr := &dns.SPF{Hdr: h, Txt: []string{spf}}
						record.RR = rr
					} else {
						log.Printf("Zero length SPF record for '%s' in '%s'\n", label.Label, zone.Origin)
						continue
					}

				default:
					log.Println("type:", rType)
					panic("Don't know how to handle this type")
				}

				if record.RR == nil {
					panic("record.RR is nil")
				}

				label.Weight[dnsType] += record.Weight
				label.Records[dnsType][i] = record
			}
			if label.Weight[dnsType] > 0 {
				sort.Sort(RecordsByWeight{label.Records[dnsType]})
			}
		}
	}

	// Loop over exisiting labels, create zone records for missing sub-domains
	// and set TTLs
	for k := range zone.Labels {
		if strings.Contains(k, ".") {
			subLabels := strings.Split(k, ".")
			for i := 1; i < len(subLabels); i++ {
				subSubLabel := strings.Join(subLabels[i:], ".")
				if _, ok := zone.Labels[subSubLabel]; !ok {
					zone.AddLabel(subSubLabel)
				}
			}
		}
		for _, records := range zone.Labels[k].Records {
			for _, r := range records {
				// We add the TTL as a last pass because we might not have
				// processed it yet when we process the record data.

				var defaultTtl uint32 = 86400
				if r.RR.Header().Rrtype != dns.TypeNS {
					// NS records have special treatment. If they are not specified, they default to 86400 rather than
					// defaulting to the zone ttl option. The label TTL option always works though
					defaultTtl = uint32(zone.Options.Ttl)
				}
				if zone.Labels[k].Ttl > 0 {
					defaultTtl = uint32(zone.Labels[k].Ttl)
				}
				if r.RR.Header().Ttl == 0 {
					r.RR.Header().Ttl = defaultTtl
				}
			}
		}
	}

	zone.addSOA()

}

func getStringWeight(rec []interface{}) (string, int) {
	str := rec[0].(string)
	var weight int

	if len(rec) > 1 {
		switch rec[1].(type) {
		case string:
			var err error
			weight, err = strconv.Atoi(rec[1].(string))
			if err != nil {
				panic("Error converting weight to integer")
			}
		case float64:
			weight = int(rec[1].(float64))
		}
	}

	return str, weight
}
