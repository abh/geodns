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
}
