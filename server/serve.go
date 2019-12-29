package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geodns/applog"
	"github.com/abh/geodns/edns"
	"github.com/abh/geodns/querylog"
	"github.com/abh/geodns/zones"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

func getQuestionName(z *zones.Zone, fqdn string) string {
	lx := dns.SplitDomainName(fqdn)
	ql := lx[0 : len(lx)-z.LabelCount]
	return strings.ToLower(strings.Join(ql, "."))
}

func (srv *Server) serve(w dns.ResponseWriter, req *dns.Msg, z *zones.Zone) {

	qnamefqdn := req.Question[0].Name
	qtype := req.Question[0].Qtype

	var qle *querylog.Entry

	if srv.queryLogger != nil {
		qle = &querylog.Entry{
			Time:   time.Now().UnixNano(),
			Origin: z.Origin,
			Name:   strings.ToLower(qnamefqdn),
			Qtype:  qtype,
		}
		defer srv.queryLogger.Write(qle)
	}

	applog.Printf("[zone %s] incoming  %s %s (id %d) from %s\n", z.Origin, qnamefqdn,
		dns.TypeToString[qtype], req.Id, w.RemoteAddr())

	applog.Println("Got request", req)

	// qlabel is the qname without the zone origin suffix
	qlabel := getQuestionName(z, qnamefqdn)

	z.Metrics.LabelStats.Add(qlabel)

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

	var ip net.IP // EDNS CLIENT SUBNET or real IP
	var ecs *dns.EDNS0_SUBNET

	if option := req.IsEdns0(); option != nil {
		for _, s := range option.Option {
			switch e := s.(type) {
			case *dns.EDNS0_SUBNET:
				applog.Println("Got edns-client-subnet", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
				if e.Address != nil {
					ecs = e
					ip = e.Address

					if qle != nil {
						qle.HasECS = true
						qle.ClientAddr = fmt.Sprintf("%s/%d", ip, e.SourceNetmask)
					}
				}
			}
		}
	}

	if len(ip) == 0 { // no edns client subnet
		ip = realIP
		if qle != nil {
			qle.ClientAddr = fmt.Sprintf("%s/%d", ip, len(ip)*8)
		}
	}

	targets, netmask, location := z.Options.Targeting.GetTargets(ip, z.HasClosest)

	m := &dns.Msg{}

	// setup logging of answers and rcode
	if qle != nil {
		qle.Targets = targets
		defer func() {
			qle.Rcode = m.Rcode
			qle.Answers = len(m.Answer)
		}()
	}

	mv, err := edns.Version(req)
	if err != nil {
		m = mv
		err := w.WriteMsg(m)
		if err != nil {
			applog.Printf("could not write response: %s", err)
		}
		return
	}

	m.SetReply(req)

	if option := edns.SetSizeAndDo(req, m); option != nil {

		for _, s := range option.Option {
			switch e := s.(type) {
			case *dns.EDNS0_NSID:
				e.Code = dns.EDNS0NSID
				e.Nsid = hex.EncodeToString([]byte(srv.info.ID))
			case *dns.EDNS0_SUBNET:
				// access e.Family, e.Address, etc.
				// TODO: set scope to 0 if there are no alternate responses
				if ecs.Family != 0 {
					if netmask < 16 {
						netmask = 16
					}
					e.SourceScope = uint8(netmask)
				}
			}
		}
	}

	m.Authoritative = true

	labelMatches := z.FindLabels(qlabel, targets, []uint16{dns.TypeMF, dns.TypeCNAME, qtype})

	if len(labelMatches) == 0 {

		permitDebug := srv.PublicDebugQueries || (realIP != nil && realIP.IsLoopback())

		firstLabel := (strings.Split(qlabel, "."))[0]

		if qle != nil {
			qle.LabelName = firstLabel
		}

		if permitDebug && firstLabel == "_status" {
			if qtype == dns.TypeANY || qtype == dns.TypeTXT {
				m.Answer = srv.statusRR(qlabel + "." + z.Origin + ".")
			} else {
				m.Ns = append(m.Ns, z.SoaRR())
			}
			m.Authoritative = true
			w.WriteMsg(m)
			return
		}

		if permitDebug && firstLabel == "_health" {
			if qtype == dns.TypeANY || qtype == dns.TypeTXT {
				baseLabel := strings.Join((strings.Split(qlabel, "."))[1:], ".")
				m.Answer = z.HealthRR(qlabel+"."+z.Origin+".", baseLabel)
				m.Authoritative = true
				w.WriteMsg(m)
				return
			}
			m.Ns = append(m.Ns, z.SoaRR())
			m.Authoritative = true
			w.WriteMsg(m)
			return
		}

		if firstLabel == "_country" {
			if qtype == dns.TypeANY || qtype == dns.TypeTXT {
				h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
				h.Name = qnamefqdn

				txt := []string{
					w.RemoteAddr().String(),
					ip.String(),
				}

				targets, netmask, location := z.Options.Targeting.GetTargets(ip, z.HasClosest)
				txt = append(txt, strings.Join(targets, " "))
				txt = append(txt, fmt.Sprintf("/%d", netmask), srv.info.ID, srv.info.IP)
				if location != nil {
					txt = append(txt, fmt.Sprintf("(%.3f,%.3f)", location.Latitude, location.Longitude))
				} else {
					txt = append(txt, "()")
				}

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
		srv.metrics.Queries.With(
			prometheus.Labels{
				"zone":  z.Origin,
				"qtype": dns.TypeToString[qtype],
				"qname": "_error",
				"rcode": dns.RcodeToString[m.Rcode],
			}).Inc()
		m.Authoritative = true

		m.Ns = []dns.RR{z.SoaRR()}

		w.WriteMsg(m)
		return
	}

	for _, match := range labelMatches {
		label := match.Label
		labelQtype := match.Type

		if !label.Closest {
			location = nil
		}

		if servers := z.Picker(label, labelQtype, label.MaxHosts, location); servers != nil {
			var rrs []dns.RR
			for _, record := range servers {
				rr := dns.Copy(record.RR)
				rr.Header().Name = qnamefqdn
				rrs = append(rrs, rr)
			}
			m.Answer = rrs
		}
		if len(m.Answer) > 0 {
			// maxHosts only matter within a "targeting group"; at least that's
			// how it has been working, so we stop looking for answers as soon
			// as we have some.

			if qle != nil {
				qle.LabelName = label.Label
				qle.Answers = len(m.Answer)
			}

			break
		}
	}

	if len(m.Answer) == 0 {
		// Return a SOA so the NOERROR answer gets cached
		m.Ns = append(m.Ns, z.SoaRR())
	}

	srv.metrics.Queries.With(
		prometheus.Labels{
			"zone":  z.Origin,
			"qtype": dns.TypeToString[qtype],
			"qname": qlabel,
			"rcode": dns.RcodeToString[m.Rcode],
		}).Inc()

	applog.Println(m)

	if qle != nil {
		// should this be in the match loop above?
		qle.Rcode = m.Rcode
	}
	err = w.WriteMsg(m)
	if err != nil {
		// if Pack'ing fails the Write fails. Return SERVFAIL.
		applog.Printf("Error writing packet: %q, %s", err, m)
		dns.HandleFailed(w, req)
	}
	return
}

func (srv *Server) statusRR(label string) []dns.RR {
	h := dns.RR_Header{Ttl: 1, Class: dns.ClassINET, Rrtype: dns.TypeTXT}
	h.Name = label

	status := map[string]string{"v": srv.info.Version, "id": srv.info.ID}

	hostname, err := os.Hostname()
	if err == nil {
		status["h"] = hostname
	}

	status["up"] = strconv.Itoa(int(time.Since(srv.info.Started).Seconds()))

	js, err := json.Marshal(status)

	return []dns.RR{&dns.TXT{Hdr: h, Txt: []string{string(js)}}}
}
