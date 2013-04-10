package main

import (
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net/http"
	"time"
)

type MonitorSuite struct {
	zones Zones
}

var _ = Suite(&MonitorSuite{})

func (s *MonitorSuite) SetUpSuite(c *C) {
	s.zones = make(Zones)

	fmt.Println("Starting http server")

	zonesReadDir("dns", s.zones)
	go httpHandler(s.zones)
	time.Sleep(500 * time.Millisecond)
}

func (s *MonitorSuite) TestMonitorVersion(c *C) {
	c.Check(true, DeepEquals, true)

	res, err := http.Get("http://localhost:8053/version")
	c.Assert(err, IsNil)
	page, _ := ioutil.ReadAll(res.Body)
	c.Check(string(page), Matches, ".*<title>GeoDNS [0-9].*")

}
