package zones

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestReadConfigs(t *testing.T) {

	muxm, err := NewMuxManager("../dns", &NilReg{})
	if err != nil {
		t.Logf("loading zones: %s", err)
		t.Fail()
	}

	// Just check that example.com and test.example.org loaded, too.
	for _, zonename := range []string{"example.com", "test.example.com"} {

		if z, ok := muxm.zonelist[zonename]; ok {
			if z.Origin != zonename {
				t.Logf("zone '%s' doesn't have that Origin '%s'", zonename, z.Origin)
				t.Fail()
			}
			if z.Options.Serial == 0 {
				t.Logf("Zone '%s' Serial number is 0, should be set by file timestamp", zonename)
				t.Fail()
			}
		} else {
			t.Fatalf("Didn't load '%s'", zonename)
		}
	}

	// The real tests are in test.example.com so we have a place
	// to make nutty configuration entries
	tz := muxm.zonelist["test.example.com"]

	// test.example.com was loaded

	if tz.Options.MaxHosts != 2 {
		t.Logf("MaxHosts=%d, expected 2", tz.Options.MaxHosts)
		t.Fail()
	}

	if tz.Options.Contact != "support.bitnames.com" {
		t.Logf("Contact='%s', expected support.bitnames.com", tz.Options.Contact)
		t.Fail()
	}
	// c.Check(tz.Options.Targeting.String(), Equals, "@ continent country regiongroup region asn ip")

	// // Got logging option
	// c.Check(tz.Logging.StatHat, Equals, true)

	// c.Check(tz.Labels["weight"].MaxHosts, Equals, 1)

	// /* test different cname targets */
	// c.Check(tz.Labels["www"].
	// 	FirstRR(dns.TypeCNAME).(*dns.CNAME).
	// 	Target, Equals, "geo.bitnames.com.")

	// c.Check(tz.Labels["www-cname"].
	// 	FirstRR(dns.TypeCNAME).(*dns.CNAME).
	// 	Target, Equals, "bar.test.example.com.")

	// c.Check(tz.Labels["www-alias"].
	// 	FirstRR(dns.TypeMF).(*dns.MF).
	// 	Mf, Equals, "www")

	// // The header name should just have a dot-prefix
	// c.Check(tz.Labels[""].Records[dns.TypeNS][0].RR.(*dns.NS).Hdr.Name, Equals, "test.example.com.")

}

func TestRemoveConfig(t *testing.T) {
	dir, err := ioutil.TempDir("", "geodns-test.")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(dir)

	muxm, err := NewMuxManager(dir, &NilReg{})
	if err != nil {
		t.Logf("loading zones: %s", err)
		t.Fail()
	}

	muxm.reload()

	_, err = CopyFile("../dns/test.example.org.json", dir+"/test.example.org.json")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	_, err = CopyFile("../dns/test.example.org.json", dir+"/test2.example.org.json")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	err = ioutil.WriteFile(dir+"/invalid.example.org.json", []byte("not-json"), 0644)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	muxm.reload()
	if muxm.zonelist["test.example.org"].Origin != "test.example.org" {
		t.Log("test.example.org has unexpected Origin: '%s'", muxm.zonelist["test.example.org"].Origin)
		t.Fail()
	}
	if muxm.zonelist["test2.example.org"].Origin != "test2.example.org" {
		t.Log("test2.example.org has unexpected Origin: '%s'", muxm.zonelist["test2.example.org"].Origin)
		t.Fail()
	}

	os.Remove(dir + "/test2.example.org.json")
	os.Remove(dir + "/invalid.example.org.json")

	muxm.reload()

	if muxm.zonelist["test.example.org"].Origin != "test.example.org" {
		t.Log("test.example.org has unexpected Origin: '%s'", muxm.zonelist["test.example.org"].Origin)
		t.Fail()
	}
	_, ok := muxm.zonelist["test2.example.org"]
	if ok != false {
		t.Log("test2.example.org is still loaded")
		t.Fail()
	}
}

func CopyFile(src, dst string) (int64, error) {
	sf, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("Could not copy '%s' to '%s': %s", src, dst, err)
	}
	defer sf.Close()
	df, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("Could not copy '%s' to '%s': %s", src, dst, err)
	}
	defer df.Close()
	return io.Copy(df, sf)
}
