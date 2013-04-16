package main

import (
	"encoding/json"
	"fmt"
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

	// Global meter
	qCounter.Mark(1)

	// Zone meter
	z.Metrics.Queries.Mark(1)

	logPrintln("Got request", req)

	label := getQuestionName(z, req)

	z.Metrics.LabelStats.Add(label)

	realIp, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	z.Metrics.ClientStats.Add(realIp)

	var ip string // EDNS or real IP
	var edns *dns.EDNS0_SUBNET
	var opt_rr *dns.OPT

	for _, extra := range req.Extra {

		switch extra.(type) {
		case *dns.OPT:
			for _, o := range extra.(*dns.OPT).Option {
				opt_rr = extra.(*dns.OPT)
				switch e := o.(type) {
				case *dns.EDNS0_NSID:
					// do stuff with e.Nsid
				case *dns.EDNS0_SUBNET:
					z.Metrics.EdnsQueries.Mark(1)
					logPrintln("Got edns", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
					if e.Address != nil {
						edns = e
						ip = e.Address.String()
					}
				}
			}
		}
	}

	if len(ip) == 0 { // no edns subnet
		ip = realIp
	}

	var country string
	var netmask int
	if geoIP != nil {
		country, netmask = geoIP.GetCountry(ip)
		country = strings.ToLower(country)
	}

	m := new(dns.Msg)
	m.SetReply(req)
	if e := m.IsEdns0(); e != nil {
		m.SetEdns0(4096, e.Do())
	}
	m.Authoritative = true

	// TODO: set scope to 0 if there are no alternate responses
	if edns != nil {
		if edns.Family != 0 {
			if netmask < 16 {
				netmask = 16
			}
			edns.SourceScope = uint8(netmask)
			m.Extra = append(m.Extra, opt_rr)
		}
	}

	labels, labelQtype := z.findLabels(label, country, qTypes{dns.TypeMF, dns.TypeCNAME, qtype})
	if labelQtype == 0 {
		labelQtype = qtype
	}

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

	if servers := labels.Picker(labelQtype, labels.MaxHosts); servers != nil {
		var rrs []dns.RR
		for _, record := range servers {
			rr := record.RR
			rr.Header().Name = req.Question[0].Name
			rrs = append(rrs, rr)
		}
		m.Answer = rrs
	}

	if len(m.Answer) == 0 {
		m.Ns = append(m.Ns, z.SoaRR())
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
	status["qs"] = strconv.FormatInt(qCounter.Count(), 10)
	status["qps1"] = fmt.Sprintf("%.4f", qCounter.Rate1())

	js, err := json.Marshal(status)

	return []dns.RR{&dns.TXT{Hdr: h, Txt: []string{string(js)}}}
}

func setupServerFunc(Zone *Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		serve(w, r, Zone)
	}
}

func listenAndServe(ip string) {

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
