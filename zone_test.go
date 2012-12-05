package main

import (
	"github.com/miekg/dns"
	. "launchpad.net/gocheck"
)

func (s *ConfigSuite) TestZone(c *C) {
	ex := s.zones["example.com"]
	c.Check(ex.Labels["weight"].MaxHosts, Equals, 1)

	// Make sure that the empty "no.bar" zone gets skipped and "bar" is used
	label := ex.findLabels("bar", "no", dns.TypeA)
	c.Check(label.Records[dns.TypeA], HasLen, 1)
	c.Check(label.Records[dns.TypeA][0].RR.(*dns.RR_A).A.String(), Equals, "192.168.1.2")

	label = ex.findLabels("", "", dns.TypeMX)
	Mxs := label.Records[dns.TypeMX]
	c.Check(Mxs, HasLen, 2)
	c.Check(Mxs[0].RR.(*dns.RR_MX).Mx, Equals, "mx.example.net.")
	c.Check(Mxs[1].RR.(*dns.RR_MX).Mx, Equals, "mx2.example.net.")

	Mxs = ex.findLabels("", "dk", dns.TypeMX).Records[dns.TypeMX]
	c.Check(Mxs, HasLen, 1)
	c.Check(Mxs[0].RR.(*dns.RR_MX).Mx, Equals, "mx-eu.example.net.")

}
