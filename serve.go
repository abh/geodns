package main

import (
	"fmt"
	"github.com/miekg/dns"
	"log"
	"os"
	"os/signal"
	"strings"
)

func getQuestionName(z *Zone, req *dns.Msg) string {
	lx := dns.SplitLabels(req.Question[0].Name)
	ql := lx[0 : len(lx)-z.LenLabels-1]
	return strings.Join(ql, ".")
}

var geoIP = setupGeoIP()

func serve(w dns.ResponseWriter, req *dns.Msg, z *Zone) {

	qtype := req.Question[0].Qtype

	logPrintf("[zone %s] incoming %s %s %d from %s\n", z.Origin, req.Question[0].Name,
		dns.Rr_str[qtype], req.MsgHdr.Id, w.RemoteAddr())

	//fmt.Printf("ZONE DATA  %#v\n", z)

	fmt.Println("Got request", req)

	label := getQuestionName(z, req)

	raddr := w.RemoteAddr()

	var country *string
	if geoIP != nil {
		country = geoIP.GetCountry(raddr.String())
		fmt.Println("Country:", country)
	}

	m := new(dns.Msg)
	m.SetReply(req)
	ednsFromRequest(req, m)

	m.MsgHdr.Authoritative = true
	m.Authoritative = true

	labels := z.findLabels(label, *country, qtype)
	if labels == nil {
		// return NXDOMAIN
		m.SetRcode(req, dns.RcodeNameError)
		m.Authoritative = true

		m.Ns = []dns.RR{z.Labels[""].Records[dns.TypeSOA][0].RR}

		w.Write(m)
		return
	}

	fmt.Println("Has the label, looking for records")

	if servers := labels.Picker(qtype, 4); servers != nil {
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

	if len(m.Answer) == 0 {
		m.Ns = append(m.Ns, z.Labels[""].Records[dns.TypeSOA][0].RR)
	}

	fmt.Println("Writing reply")

	w.Write(m)
	return
}

func setupServer(Zone Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		serve(w, r, &Zone)
	}
}

func runServe(Zones *Zones) {

	for zoneName, Zone := range *Zones {
		dns.HandleFunc(zoneName, setupServer(*Zone))
	}
	// Only listen on UDP for now
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
