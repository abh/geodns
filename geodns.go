package main

import (
	"dns"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
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

func serve(w dns.ResponseWriter, req *dns.Msg, z *Zone, opt *Options) {
	logPrintf("[zone %s] incoming %s %s %d from %s\n", z.Origin, req.Question[0].Name, dns.Rr_str[req.Question[0].Qtype], req.MsgHdr.Id, w.RemoteAddr())

	fmt.Println("Got request", req)

	m := new(dns.Msg)
	m.SetReply(req)
	m.MsgHdr.Authoritative = true

	// TODO: Function to find appropriate label with records
	if region, ok := z.Labels[""]; ok {
		if region_rr := region.Records[req.Question[0].Qtype]; region_rr != nil {
			//fmt.Printf("REGION_RR %T %v\n", region_rr, region_rr)
			max := len(region_rr)
			if max > 4 {
				max = 4
			}
			servers := region_rr[0:max]
			var rrs []dns.RR
			for _, record := range servers {
				rr := record.RR
				rr.Header().Name = req.Question[0].Name
				fmt.Println(rr)
				rrs = append(rrs, rr)
			}
			m.Answer = rrs
		}
	}

	ednsFromRequest(req, m)
	w.Write(m)
	return
}

func ednsFromRequest(req, m *dns.Msg) {
	for _, r := range req.Extra {
		if r.Header().Rrtype == dns.TypeOPT {
			m.SetEdns0(4096, r.(*dns.RR_OPT).Do())
			return
		}
	}
	return
}

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

				// fmt.Println("V", v)

				var data map[string]interface{}
				data = v.(map[string]interface{})
				//fmt.Println("DATA", data)

				for dk, dv := range data {

					fmt.Printf("K %s V %s TYPE-V %T\n", dk, dv, dv)

					Zone.Labels[dk] = new(Label)
					label := Zone.Labels[dk]
					//make([]Server, len(Records))

					var a = dv.(map[string]interface{})["a"]

					if a == nil {
						fmt.Println("No A records, continue..")
						continue
					}

					//					fmt.Println("A", a)
					fmt.Printf("A %s TYPE-A %T\n", a, a)

					Records := make(map[string][]interface{})

					Records["a"] = a.([]interface{})

					//fmt.Printf("RECORDS %s TYPE-REC %T\n", Records, Records)

					if label.Records == nil {
						label.Records = make(map[uint16][]Record)
					}

					label.Records[dns.TypeA] = make([]Record, len(Records["a"]))

					for i := 0; i < len(Records["a"]); i++ {
						foo := Records["a"][i].([]interface{})
						//fmt.Printf("FOO TYPE %T %s\n", foo, foo)
						record := new(Record)
						ip := foo[0].(string)

						record.Weight, err = strconv.Atoi(foo[1].(string))

						var h dns.RR_Header
						h.Ttl = uint32(Options.Ttl)
						h.Class = dns.ClassINET

						h.Rrtype = dns.TypeA

						rr := new(dns.RR_A)
						rr.Hdr = h
						rr.A = net.ParseIP(ip)
						if rr.A == nil {
							panic("Bad A record")
						}
						record.RR = rr
						//fmt.Println(rr)

						label.Records[dns.TypeA][i] = *record
					}
				}
				//fmt.Println(Zones[k])
			}
		}

	}

	//fmt.Printf("ZO T: %T %s\n", Zones["0.us"], Zones["0.us"])

	//fmt.Println("IP", string(Zone.Regions["0.us"].IPv4[0].ip))

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) { serve(w, r, Zone, Options) })
	// Only listen on UDP
	go func() {
		if err := dns.ListenAndServe(*listen, "udp", nil); err != nil {
			log.Fatalf("geodns: failed to setup %s %s", *listen, "udp")
		}
	}()

	if *flagrun {

		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt)

	forever:
		for {
			select {
			case <-sig:
				log.Printf("geodns: signal received, stopping")
				break forever
			}
		}
	}
}
