package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geodns/querylog"
	"github.com/miekg/dns"
	"github.com/rcrowley/go-metrics"
)

func getQuestionName(z *Zone, req *dns.Msg) string {
	lx := dns.SplitDomainName(req.Question[0].Name)
	ql := lx[0 : len(lx)-z.LabelCount]
	return strings.ToLower(strings.Join(ql, "."))
}

func (srv *Server) serve(w dns.ResponseWriter, req *dns.Msg, z *Zone) {

	qname := req.Question[0].Name
	qtype := req.Question[0].Qtype

	var qle *querylog.Entry

	if srv.queryLogger != nil {
		qle = &querylog.Entry{
			Time:   time.Now().UnixNano(),
			Origin: z.Origin,
			Name:   qname,
			Qtype:  qtype,
		}
		defer srv.queryLogger.Write(qle)
	}

	logPrintf("[zone %s] incoming  %s %s (id %d) from %s\n", z.Origin, qname,
		dns.TypeToString[qtype], req.Id, w.RemoteAddr())

	// Global meter
	metrics.Get("queries").(metrics.Meter).Mark(1)

	// Zone meter
	z.Metrics.Queries.Mark(1)

	logPrintln("Got request", req)

	label := getQuestionName(z, req)

	z.Metrics.LabelStats.Add(label)

	// IP that's talking to us (not EDNS CLIENT SUBNET)
	var realIP net.IP

	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	} else if addr, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	}
	if qle != nil {
		qle.RemoteAddr = realIP.String()
	}

	z.Metrics.ClientStats.Add(realIP.String())

	var ip net.IP // EDNS or real IP
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
						ip = e.Address

						if qle != nil {
							qle.HasECS = true
							qle.ClientAddr = fmt.Sprintf("%s/%d", ip, e.SourceNetmask)
						}
					}
				}
			}
		}
	}

	if len(ip) == 0 { // no edns subnet
		ip = realIP
		if qle != nil {
			qle.ClientAddr = fmt.Sprintf("%s/%d", ip, len(ip)*8)
		}
	}

	targets, netmask := z.Options.Targeting.GetTargets(ip)

	if qle != nil {
		qle.Targets = targets
	}

	m := new(dns.Msg)

	if qle != nil {
		defer func() {
			qle.Rcode = m.Rcode
			qle.Answers = len(m.Answer)
		}()
	}

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

	labels, labelQtype := z.findLabels(label, targets, qTypes{dns.TypeMF, dns.TypeCNAME, qtype})
	if labelQtype == 0 {
		labelQtype = qtype
	}

	if labels == nil {

		permitDebug := !*flagPrivateDebug || (realIP != nil && realIP.IsLoopback())

		firstLabel := (strings.Split(label, "."))[0]

		if qle != nil {
			qle.LabelName = firstLabel
		}

		if permitDebug && firstLabel == "_status" {
			if qtype == dns.TypeANY || qtype == dns.TypeTXT {
				m.Answer = statusRR(label + "." + z.Origin + ".")
			} else {
				m.Ns = append(m.Ns, z.SoaRR())
			}
			m.Authoritative = true
			w.WriteMsg(m)
			return
		}

		if firstLabel == "_country" {
			if qtype == dns.TypeANY || qtype == dns.TypeTXT {
				h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
				h.Name = label + "." + z.Origin + "."

				txt := []string{
					w.RemoteAddr().String(),
					ip.String(),
				}

				targets, netmask := z.Options.Targeting.GetTargets(ip)
				txt = append(txt, strings.Join(targets, " "))
				txt = append(txt, fmt.Sprintf("/%d", netmask), serverID, serverIP)

				m.Answer = []dns.RR{&dns.TXT{Hdr: h,
					Txt: txt,
				}}
			} else {
				m.Ns = append(m.Ns, z.SoaRR())
			}

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
			rr := dns.Copy(record.RR)
			rr.Header().Name = qname
			rrs = append(rrs, rr)
		}
		m.Answer = rrs
	}

	if len(m.Answer) == 0 {
		// Return a SOA so the NOERROR answer gets cached
		m.Ns = append(m.Ns, z.SoaRR())
	}

	logPrintln(m)

	if qle != nil {
		qle.LabelName = labels.Label
		qle.Answers = len(m.Answer)
		qle.Rcode = m.Rcode
	}
	err := w.WriteMsg(m)
	if err != nil {
		// if Pack'ing fails the Write fails. Return SERVFAIL.
		log.Println("Error writing packet", m)
		dns.HandleFailed(w, req)
	}
	return
}

func statusRR(label string) []dns.RR {
	h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
	h.Name = label

	status := map[string]string{"v": VERSION, "id": serverID}

	hostname, err := os.Hostname()
	if err == nil {
		status["h"] = hostname
	}

	qCounter := metrics.Get("queries").(metrics.Meter)
	status["up"] = strconv.Itoa(int(time.Since(timeStarted).Seconds()))
	status["qs"] = strconv.FormatInt(qCounter.Count(), 10)
	status["qps1"] = fmt.Sprintf("%.4f", qCounter.Rate1())

	js, err := json.Marshal(status)

	return []dns.RR{&dns.TXT{Hdr: h, Txt: []string{string(js)}}}
}
