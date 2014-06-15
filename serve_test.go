package main

import (
	"net"
	"strings"
	"time"

	"github.com/abh/dns"
	. "gopkg.in/check.v1"
)

const (
	PORT = ":8853"
)

type ServeSuite struct {
}

var _ = Suite(&ServeSuite{})

func (s *ServeSuite) SetUpSuite(c *C) {

	// setup and register metrics
	metrics := NewMetrics()
	go metrics.Updater(false)

	Zones := make(Zones)
	setupPgeodnsZone(Zones)
	zonesReadDir("dns", Zones)

	go listenAndServe(PORT)

	time.Sleep(200 * time.Millisecond)
}

func (s *ServeSuite) TestServing(c *C) {

	r := exchange(c, "_status.pgeodns.", dns.TypeTXT)
	txt := r.Answer[0].(*dns.TXT).Txt[0]
	if !strings.HasPrefix(txt, "{") {
		c.Log("Unexpected result for _status.pgeodns", txt)
		c.Fail()
	}

	// Allow _country and _status queries as long as the first label is that
	r = exchange(c, "_country.foo.pgeodns.", dns.TypeTXT)
	txt = r.Answer[0].(*dns.TXT).Txt[0]
	// Got appropriate response for _country txt query
	if !strings.HasPrefix(txt, "127.0.0.1:") {
		c.Log("Unexpected result for _country.foo.pgeodns", txt)
		c.Fail()
	}

	// Make sure A requests for _status doesn't NXDOMAIN
	r = exchange(c, "_status.pgeodns.", dns.TypeA)
	c.Check(r.Answer, HasLen, 0)
	// Got one SOA record
	c.Check(r.Ns, HasLen, 1)
	// NOERROR for A request
	c.Check(r.Rcode, Equals, dns.RcodeSuccess)

	r = exchange(c, "bar.test.example.com.", dns.TypeA)
	ip := r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.2")

	r = exchange(c, "test.example.com.", dns.TypeSOA)
	soa := r.Answer[0].(*dns.SOA)
	serial := soa.Serial
	c.Check(int(serial), Equals, 3)

	// no AAAA records for 'bar', so check we get a soa record back
	r = exchange(c, "test.example.com.", dns.TypeAAAA)
	soa2 := r.Ns[0].(*dns.SOA)
	c.Check(soa, DeepEquals, soa2)

	// CNAMEs
	r = exchange(c, "www.test.example.com.", dns.TypeA)
	c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")

	//SPF
	r = exchange(c, "test.example.com.", dns.TypeSPF)
	c.Check(r.Answer[0].(*dns.SPF).Txt[0], Equals, "v=spf1 ~all")

	//SRV
	r = exchange(c, "_sip._tcp.test.example.com.", dns.TypeSRV)
	c.Check(r.Answer[0].(*dns.SRV).Target, Equals, "sipserver.example.com.")
	c.Check(r.Answer[0].(*dns.SRV).Port, Equals, uint16(5060))
	c.Check(r.Answer[0].(*dns.SRV).Priority, Equals, uint16(10))
	c.Check(r.Answer[0].(*dns.SRV).Weight, Equals, uint16(100))

	// MX
	r = exchange(c, "test.example.com.", dns.TypeMX)
	c.Check(r.Answer[0].(*dns.MX).Mx, Equals, "mx.example.net.")
	c.Check(r.Answer[1].(*dns.MX).Mx, Equals, "mx2.example.net.")
	c.Check(r.Answer[1].(*dns.MX).Preference, Equals, uint16(20))

	// Verify the first A record was created
	r = exchange(c, "a.b.c.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.7")

	// Verify sub-labels are created
	r = exchange(c, "b.c.test.example.com.", dns.TypeA)
	c.Check(r.Answer, HasLen, 0)
	c.Check(r.Rcode, Equals, dns.RcodeSuccess)

	r = exchange(c, "c.test.example.com.", dns.TypeA)
	c.Check(r.Answer, HasLen, 0)
	c.Check(r.Rcode, Equals, dns.RcodeSuccess)

	// Verify the first A record was created
	r = exchange(c, "three.two.one.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.5")

	// Verify single sub-labels is created and no record is returned
	r = exchange(c, "two.one.test.example.com.", dns.TypeA)
	c.Check(r.Answer, HasLen, 0)
	c.Check(r.Rcode, Equals, dns.RcodeSuccess)

	// Verify the A record wasn't over written
	r = exchange(c, "one.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.6")
}

func (s *ServeSuite) TestServingMixedCase(c *C) {

	r := exchange(c, "_sTaTUs.pGEOdns.", dns.TypeTXT)
	c.Assert(r.Rcode, Equals, dns.RcodeSuccess)
	txt := r.Answer[0].(*dns.TXT).Txt[0]
	if !strings.HasPrefix(txt, "{") {
		c.Log("Unexpected result for _status.pgeodns", txt)
		c.Fail()
	}

	n := "baR.test.eXAmPLe.cOM."
	r = exchange(c, n, dns.TypeA)
	ip := r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.2")
	c.Check(r.Answer[0].Header().Name, Equals, n)

}

func (s *ServeSuite) TestCname(c *C) {
	// Cname, two possible results

	results := make(map[string]int)

	for i := 0; i < 10; i++ {
		r := exchange(c, "www.se.test.example.com.", dns.TypeA)
		target := r.Answer[0].(*dns.CNAME).Target
		results[target]++
	}

	// Two possible results from this cname
	c.Check(results, HasLen, 2)

}

func (s *ServeSuite) TestServingAliases(c *C) {
	// Alias, no geo matches
	r := exchange(c, "bar-alias.test.example.com.", dns.TypeA)
	ip := r.Answer[0].(*dns.A).A
	c.Check(ip.String(), Equals, "192.168.1.2")

	// Alias to a cname record
	r = exchange(c, "www-alias.test.example.com.", dns.TypeA)
	c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")

	// Alias returning a cname, with geo overrides
	r = exchangeSubnet(c, "www-alias.test.example.com.", dns.TypeA, "194.239.134.1")
	c.Check(r.Answer, HasLen, 1)
	if len(r.Answer) > 0 {
		c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo-europe.bitnames.com.")
	}

	// Alias to Ns records
	r = exchange(c, "sub-alias.test.example.org.", dns.TypeNS)
	c.Check(r.Answer[0].(*dns.NS).Ns, Equals, "ns1.example.com.")

}

func (s *ServeSuite) TestServingEDNS(c *C) {
	// MX test
	r := exchangeSubnet(c, "test.example.com.", dns.TypeMX, "194.239.134.1")
	c.Check(r.Answer, HasLen, 1)
	if len(r.Answer) > 0 {
		c.Check(r.Answer[0].(*dns.MX).Mx, Equals, "mx-eu.example.net.")
	}

	c.Log("Testing www.test.example.com from .dk, should match www.europe (a cname)")

	r = exchangeSubnet(c, "www.test.example.com.", dns.TypeA, "194.239.134.0")
	c.Check(r.Answer, HasLen, 1)
	if len(r.Answer) > 0 {
		// EDNS-SUBNET test (request A, respond CNAME)
		c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo-europe.bitnames.com.")
	}

}

func (s *ServeSuite) BenchmarkServing(c *C) {
	for i := 0; i < c.N; i++ {
		exchange(c, "_country.foo.pgeodns.", dns.TypeTXT)
	}
}

func exchangeSubnet(c *C, name string, dnstype uint16, ip string) *dns.Msg {
	msg := new(dns.Msg)

	msg.SetQuestion(name, dnstype)

	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	e := new(dns.EDNS0_SUBNET)
	e.Code = dns.EDNS0SUBNET
	e.SourceScope = 0
	e.Address = net.ParseIP(ip)
	e.Family = 1 // IP4
	e.SourceNetmask = net.IPv4len * 8
	o.Option = append(o.Option, e)
	msg.Extra = append(msg.Extra, o)

	c.Log("msg", msg)

	return dorequest(c, msg)
}

func exchange(c *C, name string, dnstype uint16) *dns.Msg {
	msg := new(dns.Msg)

	msg.SetQuestion(name, dnstype)
	return dorequest(c, msg)
}

func dorequest(c *C, msg *dns.Msg) *dns.Msg {
	cli := new(dns.Client)
	r, _, err := cli.Exchange(msg, "127.0.0.1"+PORT)
	if err != nil {
		c.Log("err", err)
		c.Fail()
	}
	return r
}
