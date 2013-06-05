package main

import (
	. "launchpad.net/gocheck"
)

type TargetingSuite struct {
}

var _ = Suite(&TargetingSuite{})

func (s *TargetingSuite) SetUpSuite(c *C) {
}

func (s *TargetingSuite) TestTargetString(c *C) {
	var tgt TargetOptions
	tgt = TargetGlobal + TargetCountry + TargetContinent

	str := tgt.String()
	c.Check(str, Equals, "@ continent country")
}

func (s *TargetingSuite) TestTargetParse(c *C) {

	tgt, err := parseTargets("@ foo country")
	str := tgt.String()
	c.Check(str, Equals, "@ country")
	c.Check(err.Error(), Equals, "Unknown targeting option 'foo'")

	tgt, err = parseTargets("@ continent country")
	c.Assert(err, IsNil)
	str = tgt.String()
	c.Check(str, Equals, "@ continent country")
}
