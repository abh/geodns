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
	ip := net.ParseIP("93.184.216.34")

	g, err := geoip2.New(geoip2.FindDB())
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
			[]string{"us-ma", "us", "north-america", "@"},
			"",
		},
		{
			"@ continent regiongroup country region ",
			[]string{"us-ma", "us-east", "us", "north-america", "@"},
			"",
		},
		{
			"ip",
			[]string{"[2607:f238:2::ff:4]", "[2607:f238:2::]"},
			"2607:f238:2:0::ff:4",
		},
		{
			// GeoLite2 doesn't have cities/regions for IPv6 addresses?
			"country",
			[]string{"us"},
			"2606:2800:220:1:248:1893:25c8:1946",
		},
	}

	if ok, _ := g.HasASN(); ok {
		tests = append(tests,
			test{"@ continent regiongroup country region asn ip",
				[]string{"[98.248.0.1]", "[98.248.0.0]", "as7922", "us-ca", "us-west", "us", "north-america", "@"},
				"98.248.0.1",
			},
			test{
				"country asn",
				[]string{"as8674", "se"},
				"2a01:3f0:1:3::1",
			},
		)
	}

	for _, test := range tests {
		if len(test.IP) > 0 {
			ip = net.ParseIP(test.IP)
		}

		tgt, _ = ParseTargets(test.Str)
		targets, _, _ = tgt.GetTargets(ip, false)

		t.Logf("testing %s, got %q", ip, targets)

		if !reflect.DeepEqual(targets, test.Targets) {
			t.Logf("For IP '%s' targets '%s' expected '%s', got '%s'", ip, test.Str, test.Targets, targets)
			t.Fail()
		}

	}
}
