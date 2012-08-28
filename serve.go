package main

import (
	"github.com/miekg/dns"
	"log"
	"strings"
)

func getQuestionName(z *Zone, req *dns.Msg) string {
	lx := dns.SplitLabels(req.Question[0].Name)
	ql := lx[0 : len(lx)-z.LenLabels]
	return strings.Join(ql, ".")
}

var geoIP = setupGeoIP()

func serve(w dns.ResponseWriter, req *dns.Msg, z *Zone) {

	qtype := req.Question[0].Qtype

	logPrintf("[zone %s] incoming %s %s %d from %s\n", z.Origin, req.Question[0].Name,
		dns.Rr_str[qtype], req.MsgHdr.Id, w.RemoteAddr())

	logPrintln("Got request", req)

	label := getQuestionName(z, req)

	var country string
	if geoIP != nil {
		country = geoIP.GetCountry(w.RemoteAddr().String())
		logPrintln("Country:", country)
	}

	m := new(dns.Msg)
	m.SetReply(req)
	if e := m.IsEdns0(); e != nil {
		m.SetEdns0(4096, e.Do())
	}
	m.Authoritative = true

	// TODO(ask) Fix the findLabels API to make this work better
	if alias := z.findLabels(label, "", dns.TypeMF); alias != nil &&
		alias.Records[dns.TypeMF] != nil {
		// We found an alias record, so pretend the question was for that name instead
		label = alias.firstRR(dns.TypeMF).(*dns.RR_MF).Mf
	}

	labels := z.findLabels(label, country, qtype)
	if labels == nil {
		// return NXDOMAIN
		m.SetRcode(req, dns.RcodeNameError)
		m.Authoritative = true

		m.Ns = []dns.RR{z.SoaRR()}

		w.Write(m)
		return
	}

	if servers := labels.Picker(qtype, 4); servers != nil {
		var rrs []dns.RR
		for _, record := range servers {
			rr := record.RR
			rr.Header().Name = req.Question[0].Name
			rrs = append(rrs, rr)
		}
		m.Answer = rrs
	}

	if len(m.Answer) == 0 {
		if labels := z.Labels[label]; labels != nil {
			if _, ok := labels.Records[dns.TypeCNAME]; ok {
				cname := labels.firstRR(dns.TypeCNAME)
				m.Answer = append(m.Answer, cname)
			}
		} else {
			m.Ns = append(m.Ns, z.SoaRR())
		}
	}

	logPrintln(m)

	w.Write(m)
	return
}

func setupServerFunc(Zone *Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		serve(w, r, Zone)
	}
}

func listenAndServe(Zones *Zones) {

	// Only listen on UDP for now
	log.Printf("Opening on %s %s", *listen, "udp")
	if err := dns.ListenAndServe(*listen, "udp", nil); err != nil {
		log.Fatalf("geodns: failed to setup %s %s", *listen, "udp")
	}
}
