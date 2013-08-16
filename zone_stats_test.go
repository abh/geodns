package main

import (
	. "launchpad.net/gocheck"
)

type ZoneStatsSuite struct {
}

var _ = Suite(&ZoneStatsSuite{})

func (s *ZoneStatsSuite) TestZoneStats(c *C) {
	zs := NewZoneLabelStats(4)
	c.Assert(zs, NotNil)
	c.Log("adding 4 entries")
	zs.Add("abc")
	zs.Add("foo")
	zs.Add("def")
	zs.Add("abc")
	c.Log("getting counts")
	co := zs.Counts()
	c.Check(co["abc"], Equals, 2)
	c.Check(co["foo"], Equals, 1)
	zs.Add("foo")
	co = zs.Counts()
	//	close
	c.Check(co["abc"], Equals, 1)
	c.Check(co["foo"], Equals, 2)
	zs.Close()

	zs = NewZoneLabelStats(10)
	zs.Add("a")
	zs.Add("a")
	zs.Add("a")
	zs.Add("b")
	zs.Add("c")
	zs.Add("c")
	zs.Add("d")
	zs.Add("d")
	zs.Add("e")
	zs.Add("f")

	top := zs.TopCounts(2)
	c.Check(top, HasLen, 3)
	c.Check(top[0].Label, Equals, "a")

	zs.Reset()
	c.Check(zs.Counts(), HasLen, 0)

	zs.Close()

}
