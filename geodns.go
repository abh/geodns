package main

import (
	"dns"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	//"strings"
)

type Options struct {
	Serial int
	Ttl    int
}

type Record struct {
	RR     dns.RR
	Weight int
}

type Label struct {
	Label    string
	MaxHosts int
	Ttl      int
	Records  map[uint16][]Record
}

type labels map[string]*Label

type Zone struct {
	Origin    string
	Labels    labels
	LenLabels int
	Options   Options
}

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")
)

func (z *Zone) findLabels(s string) *Label {
	if label, ok := z.Labels[s]; ok {
		return label
	}
	return nil
}

func main() {

	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	Zone := new(Zone)
	Zone.Labels = make(labels)

	// BUG(ask) Doesn't read multiple .json zone files yet
	Zone.Origin = "ntppool.org"
	Zone.LenLabels = dns.LenLabels(Zone.Origin)

	b, err := ioutil.ReadFile("ntppool.org.json")
	if err != nil {
		panic(err)
	}

	if err == nil {
		var objmap map[string]interface{}
		err := json.Unmarshal(b, &objmap)
		if err != nil {
			panic(err)
		}
		//fmt.Println(objmap)

		var data map[string]interface{}

		for k, v := range objmap {
			fmt.Printf("k: %s v: %#v, T: %T\n", k, v, v)

			switch k {
			case "ttl", "serial":
				switch option := k; option {
				case "ttl":
					Zone.Options.Ttl = int(v.(float64))
				case "serial":
					Zone.Options.Serial = int(v.(float64))
				}
				continue

			case "data":
				data = v.(map[string]interface{})
			}
		}

		setupZoneData(data, Zone)

	}

	//fmt.Printf("ZO T: %T %s\n", Zones["0.us"], Zones["0.us"])

	//fmt.Println("IP", string(Zone.Regions["0.us"].IPv4[0].ip))

	runServe(Zone)
}

func setupZoneData(data map[string]interface{}, Zone *Zone) {

	var recordTypes = map[string]uint16{
		"a":    dns.TypeA,
		"aaaa": dns.TypeAAAA,
		"ns":   dns.TypeNS,
	}

	for dk, dv := range data {

		fmt.Printf("K %s V %s TYPE-V %T\n", dk, dv, dv)

		Zone.Labels[dk] = new(Label)
		label := Zone.Labels[dk]

		// BUG(ask) Read 'ttl' value in label data

		for rType, dnsType := range recordTypes {

			fmt.Println(rType, dnsType)

			var rdata = dv.(map[string]interface{})[rType]

			if rdata == nil {
				fmt.Printf("No %s records for label %s\n", rType, dk)
				continue
			}

			fmt.Printf("rdata %s TYPE-R %T\n", rdata, rdata)

			Records := make(map[string][]interface{})

			switch rdata.(type) {
			case map[string]interface{}:
				// Handle map[ns2.example.net:<nil> ns1.example.net:<nil>]
				tmp := make([]interface{}, 0)
				for rdata_k, rdata_v := range rdata.(map[string]interface{}) {
					if rdata_v == nil {
						rdata_v = ""
					}
					tmp = append(tmp, []string{rdata_k, rdata_v.(string)})
				}
				Records[rType] = tmp
			default:
				Records[rType] = rdata.([]interface{})
			}

			//fmt.Printf("RECORDS %s TYPE-REC %T\n", Records, Records)

			if label.Records == nil {
				label.Records = make(map[uint16][]Record)
			}

			label.Records[dnsType] = make([]Record, len(Records[rType]))

			for i := 0; i < len(Records[rType]); i++ {

				fmt.Printf("RT %T %#v\n", Records[rType][i], Records[rType][i])

				record := new(Record)

				var h dns.RR_Header
				// fmt.Println("TTL OPTIONS", Zone.Options.Ttl)
				h.Ttl = uint32(Zone.Options.Ttl)
				h.Class = dns.ClassINET
				h.Rrtype = dnsType

				switch dnsType {
				case dns.TypeA, dns.TypeAAAA:
					rec := Records[rType][i].([]interface{})
					ip := rec[0].(string)
					var err error
					switch rec[1].(type) {
					case string:
						record.Weight, err = strconv.Atoi(rec[1].(string))
						if err != nil {
							panic("Error converting weight to integer")
						}
					case float64:
						record.Weight = int(rec[1].(float64))
					}
					switch dnsType {
					case dns.TypeA:
						rr := &dns.RR_A{Hdr: h}
						rr.A = net.ParseIP(ip)
						if rr.A == nil {
							panic("Bad A record")
						}
						record.RR = rr
					case dns.TypeAAAA:
						rr := &dns.RR_AAAA{Hdr: h}
						rr.AAAA = net.ParseIP(ip)
						if rr.AAAA == nil {
							panic("Bad AAAA record")
						}
						record.RR = rr
					}
				case dns.TypeNS:
					rec := Records[rType][i]
					rr := &dns.RR_NS{Hdr: h}

					switch rec.(type) {
					case string:
						rr.Ns = rec.(string)
					case []string:
						recl := rec.([]string)
						fmt.Println("RECL:", recl)
						rr.Ns = recl[0]
						if len(recl[1]) > 0 {
							fmt.Println("NS records with names syntax not supported")
						}
					default:
						fmt.Printf("Data: %T %#v\n", rec, rec)
						panic("Unrecognized NS format/syntax")
					}

					if h.Ttl < 43000 {
						h.Ttl = 43200
					}
					record.RR = rr

				default:
					fmt.Println("type:", rType)
					panic("Don't know how to handle this type")
				}

				if record.RR == nil {
					panic("record.RR is nil")
				}

				label.Records[dnsType][i] = *record
			}
		}
	}
	//fmt.Println(Zones[k])
}
