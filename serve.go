package main

import (
	"dns"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func serve(w dns.ResponseWriter, req *dns.Msg, z *Zone, opt *Options) {
	logPrintf("[zone %s] incoming %s %s %d from %s\n", z.Origin, req.Question[0].Name, dns.Rr_str[req.Question[0].Qtype], req.MsgHdr.Id, w.RemoteAddr())

	fmt.Println("Got request", req)

	raddr := w.RemoteAddr()

	gi := setupGeoIP()
	country := gi.GetCountry(raddr.String())
	fmt.Println("Country:", country)

	m := new(dns.Msg)
	m.SetReply(req)
	m.MsgHdr.Authoritative = true

	// TODO: Function to find appropriate label with records
	if region, ok := z.Labels["2"]; ok {
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
				fmt.Println("RR", rr)
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

func runServe(Zone *Zone, Options *Options) {

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

func ednsFromRequest(req, m *dns.Msg) {
	for _, r := range req.Extra {
		if r.Header().Rrtype == dns.TypeOPT {
			m.SetEdns0(4096, r.(*dns.RR_OPT).Do())
			return
		}
	}
	return
}
