package geoip

import (
	"fmt"
	"testing"

	. "gopkg.in/check.v1"
)

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) { TestingT(t) }

type GeoIPSuite struct {
}

var _ = Suite(&GeoIPSuite{})

func (s *GeoIPSuite) Testv4(c *C) {
	gi, err := Open()
	if gi == nil || err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
		return
	}

	c.Check(gi, NotNil)

	country, netmask := gi.GetCountry("64.17.254.216")
	c.Check(country, Equals, "US")
	c.Check(netmask, Equals, 17)

	country, netmask = gi.GetCountry("222.230.136.0")
	c.Check(country, Equals, "JP")
	c.Check(netmask, Equals, 16)
}

func (s *GeoIPSuite) TestOpenType(c *C) {

	SetCustomDirectory("test-db")

	// Open Country database
	gi, err := OpenType(GEOIP_COUNTRY_EDITION)
	c.Check(err, IsNil)
	c.Assert(gi, NotNil)
	country, _ := gi.GetCountry("81.2.69.160")
	c.Check(country, Equals, "GB")
}

func (s *GeoIPSuite) Benchmark_GetCountry(c *C) {
	gi, err := Open()
	if gi == nil || err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
		return
	}

	for i := 0; i < c.N; i++ {
		gi.GetCountry("207.171.7.51")
	}
}

func (s *GeoIPSuite) Testv4Record(c *C) {
	gi, err := Open("test-db/GeoIPCity.dat")
	if gi == nil || err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
		return
	}

	c.Check(gi, NotNil)

	record := gi.GetRecord("66.92.181.240")
	c.Assert(record, NotNil)
	c.Check(
		*record,
		Equals,
		GeoIPRecord{
			CountryCode:   "US",
			CountryCode3:  "USA",
			CountryName:   "United States",
			Region:        "CA",
			City:          "Fremont",
			PostalCode:    "94538",
			Latitude:      37.5079,
			Longitude:     -121.96,
			AreaCode:      510,
			MetroCode:     807,
			CharSet:       1,
			ContinentCode: "NA",
		},
	)
}

func (s *GeoIPSuite) Benchmark_GetRecord(c *C) {

	gi, err := Open("db/GeoLiteCity.dat")
	if gi == nil || err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
		return
	}

	for i := 0; i < c.N; i++ {
		record := gi.GetRecord("207.171.7.51")
		if record == nil {
			panic("")
		}
	}
}

func (s *GeoIPSuite) Testv4Region(c *C) {
	gi, err := Open("test-db/GeoIPRegion.dat")
	if gi == nil || err != nil {
		fmt.Printf("Could not open GeoIP database: %s\n", err)
		return
	}

	country, region := gi.GetRegion("64.17.254.223")
	c.Check(country, Equals, "US")
	c.Check(region, Equals, "CA")
}

func (s *GeoIPSuite) TestRegionName(c *C) {
	regionName := GetRegionName("NL", "07")
	c.Check(regionName, Equals, "Noord-Holland")
	regionName = GetRegionName("CA", "ON")
	c.Check(regionName, Equals, "Ontario")
}
