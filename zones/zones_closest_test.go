package zones

import (
	"net/netip"
	"reflect"
	"sort"
	"testing"

	dns "codeberg.org/miekg/dns"
)

func TestClosest(t *testing.T) {
	muxm := loadZones(t)
	t.Log("test closests")

	tests := []struct {
		Label     string
		ClientIP  string
		ExpectedA []string
		QType     uint16
		MaxHosts  int
	}{
		{"closest", "212.237.144.84", []string{"194.106.223.155"}, dns.TypeA, 1},
		{"closest", "208.113.157.108", []string{"207.171.7.49", "207.171.7.59"}, dns.TypeA, 2},
		{"closest", "2620:0:872::1", []string{"2607:f238:3::1:45"}, dns.TypeAAAA, 1},
		// {"closest", "208.113.157.108", []string{"207.171.7.59"}, 1},
	}

	for _, x := range tests {

		ip := netip.MustParseAddr(x.ClientIP)

		tz := muxm.zonelist["test.example.com"]
		targets, netmask, location := tz.Options.Targeting.GetTargets(ip, true)

		t.Logf("targets: %q, netmask: %d, location: %+v", targets, netmask, location)

		// This is a weird API, but it's what serve() uses now. Fixing it
		// isn't super straight forward. Moving some of the exceptions from serve()
		// into configuration and making the "find the best answer" code have
		// a better API should be possible though. Some day.
		labelMatches := tz.FindLabels(
			x.Label,
			targets,
			[]uint16{dns.TypeMF, dns.TypeCNAME, x.QType},
		)

		if len(labelMatches) == 0 {
			t.Fatalf("no labelmatches")
		}

		for _, match := range labelMatches {
			label := match.Label
			labelQtype := match.Type

			records := tz.Picker(label, labelQtype, x.MaxHosts, location)
			if records == nil {
				t.Fatalf("didn't get closest records")
			}

			if len(x.ExpectedA) == 0 {
				if len(records) > 0 {
					t.Logf("Expected 0 records but got %d", len(records))
					t.Fail()
				}
			}

			if len(x.ExpectedA) != len(records) {
				t.Logf("Expected %d records, got %d", len(x.ExpectedA), len(records))
				t.Fail()
			}

			ips := []string{}

			for _, r := range records {
				switch rr := r.RR.(type) {
				case *dns.A:
					ips = append(ips, rr.Addr.String())
				case *dns.AAAA:
					ips = append(ips, rr.Addr.String())
				default:
					t.Fatalf("unexpected RR type: %s", rr.Header().String())
				}
			}
			sort.Strings(ips)
			sort.Strings(x.ExpectedA)

			if !reflect.DeepEqual(ips, x.ExpectedA) {
				t.Logf("Got '%+v', expected '%+v'", ips, x.ExpectedA)
				t.Fail()
			}

		}
	}
}
