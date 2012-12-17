package main

import (
	"github.com/miekg/dns"
	. "launchpad.net/gocheck"
	"strings"
	"time"
)

func (s *ConfigSuite) TestServing(c *C) {

	Zones := make(Zones)
	setupPgeodnsZone(Zones)
	go configReader("dns", Zones)
	go listenAndServe(":8853", &Zones)

	time.Sleep(100 * time.Millisecond)

	r := exchange(c, "_status.pgeodns.", dns.TypeTXT)
	txt := r.Answer[0].(*dns.TXT).Txt[0]
	if !strings.HasPrefix(txt, "{") {
		c.Log("Unexpected result for _status.pgeodns", txt)
		c.Fail()
	}

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

	/* CNAMEs */
	r = exchange(c, "www.test.example.com.", dns.TypeA)
	c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")

	// TODO: make the alias and cname respond with the data for the target, too?
	r = exchange(c, "www-alias.test.example.com.", dns.TypeA)
	c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "bar-alias.test.example.com.")

	/* MX */
	r = exchange(c, "test.example.com.", dns.TypeMX)
	c.Check(r.Answer[0].(*dns.MX).Mx, Equals, "mx.example.net.")
	c.Check(r.Answer[1].(*dns.MX).Mx, Equals, "mx2.example.net.")
	c.Check(r.Answer[1].(*dns.MX).Preference, Equals, uint16(20))

}

func exchange(c *C, name string, dnstype uint16) *dns.Msg {
	msg := new(dns.Msg)
	cli := new(dns.Client)

	msg.SetQuestion(name, dnstype)
	r, _, err := cli.Exchange(msg, "127.0.0.1:8853")
	if err != nil {
		c.Log("err", err)
		c.Fail()
	}
	return r
}
