package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abh/errorutil"
	"github.com/miekg/dns"
)

// Zones maps domain names to zone data
type Zones map[string]*Zone

type ZoneReadRecord struct {
	time time.Time
	hash string
}

var lastRead = map[string]*ZoneReadRecord{}

func (srv *Server) zonesReadDir(dirName string, zones Zones) error {
	dir, err := ioutil.ReadDir(dirName)
	if err != nil {
		log.Println("Could not read", dirName, ":", err)
		return err
	}

	seenZones := map[string]bool{}

	var parseErr error

	for _, file := range dir {
		fileName := file.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".json") ||
			strings.HasPrefix(path.Base(fileName), ".") ||
			file.IsDir() {
			continue
		}

		zoneName := zoneNameFromFile(fileName)

		seenZones[zoneName] = true

		if _, ok := lastRead[zoneName]; !ok || file.ModTime().After(lastRead[zoneName].time) {
			modTime := file.ModTime()
			if ok {
				logPrintf("Reloading %s\n", fileName)
				lastRead[zoneName].time = modTime
			} else {
				logPrintf("Reading new file %s\n", fileName)
				lastRead[zoneName] = &ZoneReadRecord{time: modTime}
			}

			filename := path.Join(dirName, fileName)

			// Check the sha256 of the file has not changed. It's worth an explanation of
			// why there isn't a TOCTOU race here. Conceivably after checking whether the
			// SHA has changed, the contents then change again before we actually load
			// the JSON. This can occur in two situations:
			//
			// 1. The SHA has not changed when we read the file for the SHA, but then
			//    changes before we process the JSON
			//
			// 2. The SHA has changed when we read the file for the SHA, but then changes
			//    again before we process the JSON
			//
			// In circumstance (1) we won't reread the file the first time, but the subsequent
			// change should alter the mtime again, causing us to reread it. This reflects
			// the fact there were actually two changes.
			//
			// In circumstance (2) we have already reread the file once, and then when the
			// contents are changed the mtime changes again
			//
			// Provided files are replaced atomically, this should be OK. If files are not
			// replaced atomically we have other problems (e.g. partial reads).

			sha256 := sha256File(filename)
			if lastRead[zoneName].hash == sha256 {
				logPrintf("Skipping new file %s as hash is unchanged\n", filename)
				continue
			}

			config, err := readZoneFile(zoneName, filename)
			if config == nil || err != nil {
				parseErr = fmt.Errorf("Error reading zone '%s': %s", zoneName, err)
				log.Println(parseErr.Error())
				continue
			}

			(lastRead[zoneName]).hash = sha256

			srv.addHandler(zones, zoneName, config)
		}
	}

	for zoneName, zone := range zones {
		if zoneName == "pgeodns" {
			continue
		}
		if ok, _ := seenZones[zoneName]; ok {
			continue
		}
		log.Println("Removing zone", zone.Origin)
		delete(lastRead, zoneName)
		zone.Close()
		dns.HandleRemove(zoneName)
		delete(zones, zoneName)
	}

	return parseErr
}

func (srv *Server) setupPgeodnsZone(zones Zones) {
	zoneName := "pgeodns"
	Zone := NewZone(zoneName)
	label := new(Label)
	label.Records = make(map[uint16]Records)
	label.Weight = make(map[uint16]int)
	Zone.Labels[""] = label
	setupSOA(Zone)
	srv.addHandler(zones, zoneName, Zone)
}

func (srv *Server) setupRootZone() {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeRefused)
		w.WriteMsg(m)
	})
}

func readZoneFile(zoneName, fileName string) (zone *Zone, zerr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("reading %s failed: %s", zoneName, r)
			debug.PrintStack()
			zerr = fmt.Errorf("reading %s failed: %s", zoneName, r)
		}
	}()

	fh, err := os.Open(fileName)
	if err != nil {
		log.Printf("Could not read '%s': %s", fileName, err)
		panic(err)
	}

	zone = NewZone(zoneName)

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
		return nil, fmt.Errorf("error parsing JSON object in config file %s%s\n%v",
			fh.Name(), extra, err)
	}

	if err != nil {
		panic(err)
	}
	//log.Println(objmap)

	var data map[string]interface{}

	for k, v := range objmap {
		//log.Printf("k: %s v: %#v, T: %T\n", k, v, v)

		switch k {
		case "ttl":
			zone.Options.Ttl = valueToInt(v)
		case "serial":
			zone.Options.Serial = valueToInt(v)
		case "contact":
			zone.Options.Contact = v.(string)
		case "max_hosts":
			zone.Options.MaxHosts = valueToInt(v)
		case "targeting":
			zone.Options.Targeting, err = parseTargets(v.(string))
			if err != nil {
				log.Printf("Could not parse targeting '%s': %s", v, err)
				return nil, err
			}

		case "logging":
			{
				logging := new(ZoneLogging)
				for logger, v := range v.(map[string]interface{}) {
					switch logger {
					case "stathat":
						logging.StatHat = valueToBool(v)
					case "stathat_api":
						logging.StatHatAPI = valueToString(v)
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
	case zone.Options.Targeting >= TargetRegionGroup:
		geoIP.setupGeoIPCity()
	case zone.Options.Targeting >= TargetContinent:
		geoIP.setupGeoIPCountry()
	}
	if zone.Options.Targeting&TargetASN > 0 {
		geoIP.setupGeoIPASN()
	}

	return zone, nil
}

func setupZoneData(data map[string]interface{}, Zone *Zone) {
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

		label := Zone.AddLabel(dk)

		for rType, rdata := range dv {
			switch rType {
			case "max_hosts":
				label.MaxHosts = valueToInt(rdata)
				continue
			case "ttl":
				label.Ttl = valueToInt(rdata)
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

				// We add the TTL as a last pass because we might not have
				// processed it yet when we process the record data.

				switch len(label.Label) {
				case 0:
					h.Name = Zone.Origin + "."
				default:
					h.Name = label.Label + "." + Zone.Origin + "."
				}

				switch dnsType {
				case dns.TypeA, dns.TypeAAAA, dns.TypePTR:

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
						record.Weight = valueToInt(rec["weight"])
					}
					if rec["preference"] != nil {
						pref = uint16(valueToInt(rec["preference"]))
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
						target = target + "." + Zone.Origin
					}

					if rec["srv_weight"] != nil {
						srv_weight = uint16(valueToInt(rec["srv_weight"]))
					}
					if rec["port"] != nil {
						port = uint16(valueToInt(rec["port"]))
					}
					if rec["priority"] != nil {
						priority = uint16(valueToInt(rec["priority"]))
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
					}
					if !dns.IsFqdn(target) {
						target = target + "." + Zone.Origin
					}
					record.Weight = weight
					record.RR = &dns.CNAME{Hdr: h, Target: dns.Fqdn(target)}

				case dns.TypeMF:
					rec := records[rType][i]
					// MF records (how we store aliases) are not FQDNs
					record.RR = &dns.MF{Hdr: h, Mf: rec.(string)}

				case dns.TypeNS:
					rec := records[rType][i]
					if h.Ttl < 86400 {
						h.Ttl = 86400
					}

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
							record.Weight = valueToInt(weight)
						}
						if t, ok := recmap["txt"]; ok {
							txt = t.(string)
						}
					}
					if len(txt) > 0 {
						rr := &dns.TXT{Hdr: h, Txt: []string{txt}}
						record.RR = rr
					} else {
						log.Printf("Zero length txt record for '%s' in '%s'\n", label.Label, Zone.Origin)
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
							record.Weight = valueToInt(weight)
						}
						if t, ok := recmap["spf"]; ok {
							spf = t.(string)
						}
					}
					if len(spf) > 0 {
						rr := &dns.SPF{Hdr: h, Txt: []string{spf}}
						record.RR = rr
					} else {
						log.Printf("Zero length SPF record for '%s' in '%s'\n", label.Label, Zone.Origin)
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
				label.Records[dnsType][i] = *record
			}
			if label.Weight[dnsType] > 0 {
				sort.Sort(RecordsByWeight{label.Records[dnsType]})
			}
		}
	}

	// loop over exisiting labels, create zone records for missing sub-domains
	// and set TTLs
	for k := range Zone.Labels {
		if strings.Contains(k, ".") {
			subLabels := strings.Split(k, ".")
			for i := 1; i < len(subLabels); i++ {
				subSubLabel := strings.Join(subLabels[i:], ".")
				if _, ok := Zone.Labels[subSubLabel]; !ok {
					Zone.AddLabel(subSubLabel)
				}
			}
		}
		if Zone.Labels[k].Ttl > 0 {
			for _, records := range Zone.Labels[k].Records {
				for _, r := range records {
					r.RR.Header().Ttl = uint32(Zone.Labels[k].Ttl)
				}
			}
		}
	}

	setupSOA(Zone)

	//log.Println(Zones[k])
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

func setupSOA(Zone *Zone) {
	label := Zone.Labels[""]

	primaryNs := "ns"

	// log.Println("LABEL", label)

	if label == nil {
		log.Println(Zone.Origin, "doesn't have any 'root' records,",
			"you should probably add some NS records")
		label = Zone.AddLabel("")
	}

	if record, ok := label.Records[dns.TypeNS]; ok {
		primaryNs = record[0].RR.(*dns.NS).Ns
	}

	ttl := Zone.Options.Ttl * 10
	if ttl > 3600 {
		ttl = 3600
	}
	if ttl == 0 {
		ttl = 600
	}

	s := Zone.Origin + ". " + strconv.Itoa(ttl) + " IN SOA " +
		primaryNs + " " + Zone.Options.Contact + " " +
		strconv.Itoa(Zone.Options.Serial) +
		// refresh, retry, expire, minimum are all
		// meaningless with this implementation
		" 5400 5400 1209600 3600"

	// log.Println("SOA: ", s)

	rr, err := dns.NewRR(s)

	if err != nil {
		log.Println("SOA Error", err)
		panic("Could not setup SOA")
	}

	record := Record{RR: rr}

	label.Records[dns.TypeSOA] = make([]Record, 1)
	label.Records[dns.TypeSOA][0] = record

}

func valueToBool(v interface{}) (rv bool) {
	switch v.(type) {
	case bool:
		rv = v.(bool)
	case string:
		str := v.(string)
		switch str {
		case "true":
			rv = true
		case "1":
			rv = true
		}
	case float64:
		if v.(float64) > 0 {
			rv = true
		}
	default:
		log.Println("Can't convert", v, "to bool")
		panic("Can't convert value")
	}
	return rv

}

func valueToString(v interface{}) (rv string) {
	switch v.(type) {
	case string:
		rv = v.(string)
	case float64:
		rv = strconv.FormatFloat(v.(float64), 'f', -1, 64)
	default:
		log.Println("Can't convert", v, "to string")
		panic("Can't convert value")
	}
	return rv
}

func valueToInt(v interface{}) (rv int) {
	switch v.(type) {
	case string:
		i, err := strconv.Atoi(v.(string))
		if err != nil {
			panic("Error converting weight to integer")
		}
		rv = i
	case float64:
		rv = int(v.(float64))
	default:
		log.Println("Can't convert", v, "to integer")
		panic("Can't convert value")
	}
	return rv
}

func zoneNameFromFile(fileName string) string {
	return fileName[0:strings.LastIndex(fileName, ".")]
}

func sha256File(fn string) string {
	if data, err := ioutil.ReadFile(fn); err != nil {
		return ""
	} else {
		hasher := sha256.New()
		hasher.Write(data)
		return hex.EncodeToString(hasher.Sum(nil))
	}
}
