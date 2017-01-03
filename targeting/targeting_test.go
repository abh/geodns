package targeting

import (
	"net"
	"reflect"
	"testing"
)

func TestTargetString(t *testing.T) {
	tgt := TargetOptions(TargetGlobal + TargetCountry + TargetContinent)

	str := tgt.String()
	if str != "@ continent country" {
		t.Logf("wrong target string '%s'", str)
		t.Fail()
	}
}

func TestTargetParse(t *testing.T) {
	tgt, err := ParseTargets("@ foo country")
	str := tgt.String()
	if str != "@ country" {
		t.Logf("Expected '@ country', got '%s'", str)
		t.Fail()
	}
	if err.Error() != "Unknown targeting option 'foo'" {
		t.Log("Failed erroring on an unknown targeting option")
		t.Fail()
	}

	tests := [][]string{
		[]string{"@ continent country asn", "@ continent country asn"},
		[]string{"asn country", "country asn"},
		[]string{"continent @ country", "@ continent country"},
	}

	for _, strs := range tests {
		tgt, err = ParseTargets(strs[0])
		if err != nil {
			t.Fatalf("Parsing '%s': %s", strs[0], err)
		}
		if tgt.String() != strs[1] {
			t.Logf("Unexpected result parsing '%s', got '%s', expected '%s'",
				strs[0], tgt.String(), strs[1])
			t.Fail()
		}
	}
}

func TestGetTargets(t *testing.T) {
	ip := net.ParseIP("207.171.1.1")

	GeoIP().SetupGeoIPCity()
	GeoIP().SetupGeoIPCountry()
	GeoIP().SetupGeoIPASN()

	tgt, _ := ParseTargets("@ continent country")
	targets, _, _ := tgt.GetTargets(ip, false)
	if !reflect.DeepEqual(targets, []string{"us", "north-america", "@"}) {
		t.Fatalf("Unexpected parse results of targets")
	}

	if geoIP.city == nil {
		t.Log("City GeoIP database requred for these tests")
		return
	}

	tests := []struct {
		Str     string
		Targets []string
		IP      string
	}{
		{
			"@ continent country region ",
			[]string{"us-ca", "us", "north-america", "@"},
			"",
		},
		{
			"@ continent regiongroup country region ",
			[]string{"us-ca", "us-west", "us", "north-america", "@"},
			"",
		},
		{
			"@ continent regiongroup country region asn ip",
			[]string{"[207.171.1.1]", "[207.171.1.0]", "as7012", "us-ca", "us-west", "us", "north-america", "@"},
			"",
		},
		{
			"ip",
			[]string{"[2607:f238:2::ff:4]", "[2607:f238:2::]"},
			"2607:f238:2:0::ff:4",
		},
	}

	for _, test := range tests {
		if len(test.IP) > 0 {
			ip = net.ParseIP(test.IP)
		}

		tgt, _ = ParseTargets(test.Str)
		targets, _, _ = tgt.GetTargets(ip, false)

		if !reflect.DeepEqual(targets, test.Targets) {
			t.Logf("For targets '%s' expected '%s', got '%s'", test.Str, test.Targets, targets)
			t.Fail()
		}

	}
}
