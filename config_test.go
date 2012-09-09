package main

import (
	. "launchpad.net/gocheck"
	"testing"
)

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) { TestingT(t) }

type ConfigSuite struct {
	zones Zones
}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) TestReadConfigs(c *C) {
	s.zones = make(Zones)
	configReadDir("dns", s.zones)
	c.Check(s.zones["example.com"].Origin, Equals, "example.com")
	c.Check(s.zones["example.com"].Options.MaxHosts, Equals, 2)
	c.Check(s.zones["example.com"].Labels["weight"].MaxHosts, Equals, 1)
}
