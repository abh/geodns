package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	. "gopkg.in/check.v1"
)

type MonitorSuite struct {
	zones   Zones
	metrics *ServerMetrics
}

var _ = Suite(&MonitorSuite{})

func (s *MonitorSuite) SetUpSuite(c *C) {
	s.zones = make(Zones)
	s.metrics = NewMetrics()
	go s.metrics.Updater(false)

	*flaghttp = ":8881"

	fmt.Println("Starting http server")

	// TODO: use httptest
	// https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/Jk785WB7F8I

	zonesReadDir("dns", s.zones)
	go httpHandler(s.zones)
	time.Sleep(500 * time.Millisecond)
}

func (s *MonitorSuite) TestMonitorVersion(c *C) {
	c.Check(true, DeepEquals, true)

	res, err := http.Get("http://localhost:8881/version")
	c.Assert(err, IsNil)
	page, _ := ioutil.ReadAll(res.Body)
	c.Check(string(page), Matches, ".*<title>GeoDNS [0-9].*")

	res, err = http.Get("http://localhost:8881/status")
	c.Assert(err, IsNil)
	page, _ = ioutil.ReadAll(res.Body)
	// just check that template basically works

	isOk := strings.Contains(string(page), "<html>")
	// page has <html>
	c.Check(isOk, Equals, true)

}
