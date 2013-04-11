package main

import (
	"github.com/miekg/dns"
	. "launchpad.net/gocheck"
)

func (s *ConfigSuite) TestZone(c *C) {
	ex := s.zones["test.example.com"]

	// test.example.com was loaded
	c.Assert(ex.Labels, NotNil)

	c.Check(ex.Labels["weight"].MaxHosts, Equals, 1)

	// Make sure that the empty "no.bar" zone gets skipped and "bar" is used
	label, qtype := ex.findLabels("bar", "no", qTypes{dns.TypeA})
	c.Check(label.Records[dns.TypeA], HasLen, 1)
	c.Check(label.Records[dns.TypeA][0].RR.(*dns.A).A.String(), Equals, "192.168.1.2")
	c.Check(qtype, Equals, dns.TypeA)

	label, qtype = ex.findLabels("", "", qTypes{dns.TypeMX})
	Mxs := label.Records[dns.TypeMX]
	c.Check(Mxs, HasLen, 2)
	c.Check(Mxs[0].RR.(*dns.MX).Mx, Equals, "mx.example.net.")
	c.Check(Mxs[1].RR.(*dns.MX).Mx, Equals, "mx2.example.net.")

	label, qtype = ex.findLabels("", "dk", qTypes{dns.TypeMX})
	Mxs = label.Records[dns.TypeMX]
	c.Check(Mxs, HasLen, 1)
	c.Check(Mxs[0].RR.(*dns.MX).Mx, Equals, "mx-eu.example.net.")
	c.Check(qtype, Equals, dns.TypeMX)

	// look for multiple record types
	label, qtype = ex.findLabels("www", "", qTypes{dns.TypeCNAME, dns.TypeA})
	c.Check(label.Records[dns.TypeCNAME], HasLen, 1)
	c.Check(qtype, Equals, dns.TypeCNAME)
}
