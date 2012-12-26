package main

import (
	"encoding/json"
	"github.com/abh/geodns/countries"
	"github.com/miekg/dns"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func getQuestionName(z *Zone, req *dns.Msg) string {
	lx := dns.SplitLabels(req.Question[0].Name)
	ql := lx[0 : len(lx)-z.LenLabels]
	return strings.ToLower(strings.Join(ql, "."))
}

var geoIP = setupGeoIP()

func serve(w dns.ResponseWriter, req *dns.Msg, z *Zone) {

	qtype := req.Question[0].Qtype

	logPrintf("[zone %s] incoming %s %s %d from %s\n", z.Origin, req.Question[0].Name,
		dns.TypeToString[qtype], req.MsgHdr.Id, w.RemoteAddr())

	qCounter.Add(1)
	logPrintln("Got request", req)

	label := getQuestionName(z, req)

	var ip string
	var edns *dns.EDNS0_SUBNET
	var opt_rr *dns.OPT

	for _, extra := range req.Extra {
		log.Println("Extra", extra)
		for _, o := range extra.(*dns.OPT).Option {
			opt_rr = extra.(*dns.OPT)
			switch e := o.(type) {
			case *dns.EDNS0_NSID:
				// do stuff with e.Nsid
			case *dns.EDNS0_SUBNET:
				log.Println("========== XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
				log.Println("Got edns", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
				if e.Address != nil {
					log.Println("Setting edns to", e)
					edns = e
					ip = e.Address.String()
				}
			}
		}
	}

	var country string
	if geoIP != nil {
		if len(ip) == 0 { // no edns subnet
			ip, _, _ = net.SplitHostPort(w.RemoteAddr().String())
		}
		country = strings.ToLower(geoIP.GetCountry(ip))
		logPrintln("Country:", ip, country)
	}

	m := new(dns.Msg)
	m.SetReply(req)
	if e := m.IsEdns0(); e != nil {
		m.SetEdns0(4096, e.Do())
	}
	m.Authoritative = true

	// TODO: set scope to 0 if there are no alternate responses
	if edns != nil {
		log.Println("family", edns.Family)
		if edns.Family != 0 {
			log.Println("edns response!")
			edns.SourceScope = 16
			m.Extra = append(m.Extra, opt_rr)
		}
	}

	// TODO(ask) Fix the findLabels API to make this work better
	if alias := z.findLabels(label, "", dns.TypeMF); alias != nil &&
		alias.Records[dns.TypeMF] != nil {
		// We found an alias record, so pretend the question was for that name instead
		label = alias.firstRR(dns.TypeMF).(*dns.MF).Mf
	}

	labels := z.findLabels(label, country, qtype)
	if labels == nil {

		if label == "_status" && (qtype == dns.TypeANY || qtype == dns.TypeTXT) {
			m.Answer = statusRR(z)
			m.Authoritative = true
			w.WriteMsg(m)
			return
		}

		if label == "_country" && (qtype == dns.TypeANY || qtype == dns.TypeTXT) {
			h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
			h.Name = "_country." + z.Origin + "."

			m.Answer = []dns.RR{&dns.TXT{Hdr: h,
				Txt: []string{
					w.RemoteAddr().String(),
					ip,
					string(country),
					string(countries.CountryContinent[country]),
				},
			}}

			m.Authoritative = true
			w.WriteMsg(m)
			return
		}

		// return NXDOMAIN
		m.SetRcode(req, dns.RcodeNameError)
		m.Authoritative = true

		m.Ns = []dns.RR{z.SoaRR()}

		w.WriteMsg(m)
		return
	}

	if servers := labels.Picker(qtype, labels.MaxHosts); servers != nil {
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
			} else {
				m.Ns = append(m.Ns, z.SoaRR())
			}
		} else {
			m.Ns = append(m.Ns, z.SoaRR())
		}
	}

	logPrintln(m)

	err := w.WriteMsg(m)
	if err != nil {
		// if Pack'ing fails the Write fails. Return SERVFAIL.
		log.Println("Error writing packet", m)
		dns.HandleFailed(w, req)
	}
	return
}

func statusRR(z *Zone) []dns.RR {
	h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
	h.Name = "_status." + z.Origin + "."

	status := map[string]string{"v": VERSION, "id": serverId}

	hostname, err := os.Hostname()
	if err == nil {
		status["h"] = hostname
	}
	status["up"] = strconv.Itoa(int(time.Since(timeStarted).Seconds()))
	status["qs"] = qCounter.String()

	js, err := json.Marshal(status)

	return []dns.RR{&dns.TXT{Hdr: h, Txt: []string{string(js)}}}
}

func setupServerFunc(Zone *Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		serve(w, r, Zone)
	}
}

func listenAndServe(ip string, Zones *Zones) {

	prots := []string{"udp", "tcp"}

	for _, prot := range prots {
		go func(p string) {
			server := &dns.Server{Addr: ip, Net: p}

			log.Printf("Opening on %s %s", ip, p)
			if err := server.ListenAndServe(); err != nil {
				log.Fatalf("geodns: failed to setup %s %s: %s", ip, p, err)
			}
			log.Fatalf("geodns: ListenAndServe unexpectedly returned")
		}(prot)
	}

}
