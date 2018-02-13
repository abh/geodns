package targeting

import (
	"net"
	"reflect"
	"testing"

	"github.com/abh/geodns/targeting/geoip2"
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

	g, err := geoip2.New("../db")
	if err != nil {
		t.Fatalf("opening geoip2: %s", err)
	}
	Setup(g)

	tgt, _ := ParseTargets("@ continent country")
	targets, _, _ := tgt.GetTargets(ip, false)
	expect := []string{"us", "north-america", "@"}
	if !reflect.DeepEqual(targets, expect) {
		t.Fatalf("Unexpected parse results of targets, got '%s', expected '%s'", targets, expect)
	}

	if ok, err := g.HasLocation(); !ok {
		t.Logf("City GeoIP database required for these tests: %s", err)
		return
	}

	type test struct {
		Str     string
		Targets []string
		IP      string
	}

	tests := []test{
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
			"ip",
			[]string{"[2607:f238:2::ff:4]", "[2607:f238:2::]"},
			"2607:f238:2:0::ff:4",
		},
	}

	if ok, _ := g.HasASN(); ok {
		tests = append(tests,
			test{"@ continent regiongroup country region asn ip",
				[]string{"[207.171.1.1]", "[207.171.1.0]", "as7012", "us-ca", "us-west", "us", "north-america", "@"},
				"207.171.1.1",
			})
	}

	for _, test := range tests {
		if len(test.IP) > 0 {
			ip = net.ParseIP(test.IP)
		}

		tgt, _ = ParseTargets(test.Str)
		targets, _, _ = tgt.GetTargets(ip, false)

		if !reflect.DeepEqual(targets, test.Targets) {
			t.Logf("For IP '%s' targets '%s' expected '%s', got '%s'", ip, test.Str, test.Targets, targets)
			t.Fail()
		}

	}
}
