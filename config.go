package main

import (
	"encoding/json"
	"github.com/miekg/dns"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

var configLastRead = map[string]time.Time{}

func configReader(dirName string, Zones Zones) {
	for {
		configReadDir(dirName, Zones)
		time.Sleep(5 * time.Second)
	}
}

func configReadDir(dirName string, Zones Zones) {
	dir, err := ioutil.ReadDir(dirName)
	if err != nil {
		panic(err)
	}

	seenFiles := map[string]bool{}

	for _, file := range dir {
		fileName := file.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".json") {
			continue
		}

		seenFiles[fileName] = true

		if lastRead, ok := configLastRead[fileName]; !ok || file.ModTime().After(lastRead) {
			log.Println("Updated file, going to read", fileName)
			configLastRead[fileName] = file.ModTime()
			zoneName := fileName[0:strings.LastIndex(fileName, ".")]
			//log.Println("FILE:", i, file, zoneName)
			runtime.GC()
			config, err := readZoneFile(zoneName, path.Join(dirName, fileName))
			if config == nil || err != nil {
				log.Println("error reading file: ", err)
			}
			if config != nil && err == nil {
				Zones[zoneName] = config
				dns.HandleFunc(zoneName, setupServerFunc(config))
				runtime.GC()
			}
		}

		// TODO(ask) Disable zones not seen in two subsequent runs
	}
}

func setupPgeodnsZone(Zones Zones) {
	zoneName := "pgeodns"
	Zone := new(Zone)
	Zone.Labels = make(labels)
	Zone.Origin = zoneName
	Zone.LenLabels = dns.LenLabels(Zone.Origin)
	label := new(Label)
	label.Records = make(map[uint16]Records)
	label.Weight = make(map[uint16]int)
	Zone.Labels[""] = label
	setupSOA(Zone)
	Zones[zoneName] = Zone
	dns.HandleFunc(zoneName, setupServerFunc(Zone))
}

func readZoneFile(zoneName, fileName string) (*Zone, error) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("reading %s failed: %s", zoneName, err)
			debug.PrintStack()
		}
	}()

	fh, err := os.Open(fileName)
	if err != nil {
		log.Println("Could not read ", fileName, ": ", err)
		panic(err)
	}

	Zone := new(Zone)
	Zone.Labels = make(labels)
	Zone.Origin = zoneName
	Zone.LenLabels = dns.LenLabels(Zone.Origin)
	Zone.Options.Ttl = 120
	Zone.Options.MaxHosts = 2
	Zone.Options.Contact = "support.bitnames.com"

	if err == nil {
		var objmap map[string]interface{}
		decoder := json.NewDecoder(fh)
		err := decoder.Decode(&objmap)
		if err != nil {
			panic(err)
		}
		//log.Println(objmap)

		var data map[string]interface{}

		for k, v := range objmap {
			//log.Printf("k: %s v: %#v, T: %T\n", k, v, v)

			switch k {
			case "ttl", "serial", "max_hosts", "contact":
				switch option := k; option {
				case "ttl":
					Zone.Options.Ttl = valueToInt(v)
				case "serial":
					Zone.Options.Serial = valueToInt(v)
				case "contact":
					Zone.Options.Contact = v.(string)
				case "max_hosts":
					Zone.Options.MaxHosts = valueToInt(v)
				}
				continue

			case "data":
				data = v.(map[string]interface{})
			}
		}

		setupZoneData(data, Zone)

	}

	//log.Printf("ZO T: %T %s\n", Zones["0.us"], Zones["0.us"])

	//log.Println("IP", string(Zone.Regions["0.us"].IPv4[0].ip))

	return Zone, nil
}

func setupZoneData(data map[string]interface{}, Zone *Zone) {

	recordTypes := map[string]uint16{
		"a":     dns.TypeA,
		"aaaa":  dns.TypeAAAA,
		"ns":    dns.TypeNS,
		"cname": dns.TypeCNAME,
		"mx":    dns.TypeMX,
		"alias": dns.TypeMF,
	}

	for dk, dv_inter := range data {

		dv := dv_inter.(map[string]interface{})

		//log.Printf("K %s V %s TYPE-V %T\n", dk, dv, dv)

		label := Zone.AddLabel(dk)

		if ttl, ok := dv["ttl"]; ok {
			label.Ttl = valueToInt(ttl)
		}

		if maxHosts, ok := dv["max_hosts"]; ok {
			label.MaxHosts = valueToInt(maxHosts)
		}

		for rType, dnsType := range recordTypes {

			rdata := dv[rType]

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
				for rdata_k, rdata_v := range rdata.(map[string]interface{}) {
					if rdata_v == nil {
						rdata_v = ""
					}
					tmp = append(tmp, []string{rdata_k, rdata_v.(string)})
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
				// log.Println("TTL OPTIONS", Zone.Options.Ttl)
				h.Ttl = uint32(label.Ttl)
				h.Class = dns.ClassINET
				h.Rrtype = dnsType
				h.Name = label.Label + "." + Zone.Origin + "."

				switch dnsType {
				case dns.TypeA, dns.TypeAAAA:
					rec := records[rType][i].([]interface{})
					ip := rec[0].(string)
					var err error

					if len(rec) > 1 {
						switch rec[1].(type) {
						case string:
							record.Weight, err = strconv.Atoi(rec[1].(string))
							if err != nil {
								panic("Error converting weight to integer")
							}
						case float64:
							record.Weight = int(rec[1].(float64))
						}
					}
					switch dnsType {
					case dns.TypeA:
						if x := net.ParseIP(ip); x != nil {
							record.RR = &dns.A{Hdr: h, A: x}
							break
						}
						panic("Bad A record")
					case dns.TypeAAAA:
						if x := net.ParseIP(ip); x != nil {
							record.RR = &dns.AAAA{Hdr: h, AAAA: x}
							break
						}
						panic("Bad AAAA record")
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

				case dns.TypeCNAME:
					rec := records[rType][i]
					target := rec.(string)
					if !dns.IsFqdn(target) {
						target = target + "." + Zone.Origin
					}
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

	setupSOA(Zone)

	//log.Println(Zones[k])
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

	s := Zone.Origin + ". 3600 IN SOA " +
		primaryNs + " " + Zone.Options.Contact + " " +
		strconv.Itoa(Zone.Options.Serial) +
		" 5400 5400 2419200 " +
		strconv.Itoa(Zone.Options.Ttl)

	log.Println("SOA: ", s)

	rr, err := dns.NewRR(s)

	if err != nil {
		log.Println("SOA Error", err)
		panic("Could not setup SOA")
	}

	record := Record{RR: rr}

	label.Records[dns.TypeSOA] = make([]Record, 1)
	label.Records[dns.TypeSOA][0] = record

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
