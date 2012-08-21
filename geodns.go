package main

import (
	"dns"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
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
	Origin string
	Labels labels
}

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")
)

func main() {

	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	Zone := new(Zone)
	Zone.Labels = make(labels)

	Zone.Origin = "ntppool.org" // TODO, read multiple files etc
	Options := new(Options)

	//var objmap map[string]json.RawMessage
	var objmap map[string]interface{}

	b, err := ioutil.ReadFile("ntppool.org.json")
	if err != nil {
		panic(err)
	}

	if err == nil {
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
					Options.Ttl = int(v.(float64))
				case "serial":
					Options.Serial = int(v.(float64))
				}
				continue

			case "data":
				data = v.(map[string]interface{})
			}
		}

		setupZoneData(data, Zone, Options)

	}

	//fmt.Printf("ZO T: %T %s\n", Zones["0.us"], Zones["0.us"])

	//fmt.Println("IP", string(Zone.Regions["0.us"].IPv4[0].ip))

	runServe(Zone, Options)
}

func setupZoneData(data map[string]interface{}, Zone *Zone, Options *Options) {

	var recordTypes = map[string]uint16{
		"a":    dns.TypeA,
		"aaaa": dns.TypeAAAA,
	}

	for dk, dv := range data {

		fmt.Printf("K %s V %s TYPE-V %T\n", dk, dv, dv)

		Zone.Labels[dk] = new(Label)
		label := Zone.Labels[dk]
		//make([]Server, len(Records))

		for rType, dnsType := range recordTypes {
			fmt.Println(rType, dnsType)

			var rdata = dv.(map[string]interface{})[rType]

			if rdata == nil {
				fmt.Printf("No %s records for label %s", rType, dk)
				continue
			}

			fmt.Printf("rdata %s TYPE-R %T\n", rdata, rdata)

			Records := make(map[string][]interface{})

			Records[rType] = rdata.([]interface{})

			//fmt.Printf("RECORDS %s TYPE-REC %T\n", Records, Records)

			if label.Records == nil {
				label.Records = make(map[uint16][]Record)
			}

			label.Records[dnsType] = make([]Record, len(Records[rType]))

			for i := 0; i < len(Records[rType]); i++ {
				foo := Records[rType][i].([]interface{})
				//fmt.Printf("FOO TYPE %T %s\n", foo, foo)
				record := new(Record)
				ip := foo[0].(string)

				var err error
				record.Weight, err = strconv.Atoi(foo[1].(string))
				if err != nil {
					panic("Error converting weight to integer")
				}

				var h dns.RR_Header
				fmt.Println("TTL OPTIONS", Options.Ttl)
				h.Ttl = uint32(Options.Ttl)
				h.Class = dns.ClassINET

				h.Rrtype = dnsType

				fmt.Println("H", h)

				switch dnsType {
				case dns.TypeA:
					rr := new(dns.RR_A)
					rr.Hdr = h
					rr.A = net.ParseIP(ip)
					if rr.A == nil {
						panic("Bad A record")
					}
					record.RR = rr
				case dns.TypeAAAA:
					rr := new(dns.RR_AAAA)
					rr.Hdr = h
					rr.AAAA = net.ParseIP(ip)
					if rr.AAAA == nil {
						panic("Bad AAAA record")
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
