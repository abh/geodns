package zones

import (
	"testing"

	"github.com/miekg/dns"
)

func TestExampleComZone(t *testing.T) {
	mm, err := NewMuxManager("../dns", &NilReg{})
	if err != nil {
		t.Fatalf("Loading test zones: %s", err)
	}

	ex, ok := mm.zonelist["test.example.com"]
	if !ok || ex == nil || ex.Labels == nil {
		t.Fatalf("Did not load 'test.example.com' test zone")
	}

	if mh := ex.Labels["weight"].MaxHosts; mh != 1 {
		t.Logf("Invalid MaxHosts, expected one got '%d'", mh)
		t.Fail()
	}

	// Make sure that the empty "no.bar" zone gets skipped and "bar" is used
	label, qtype := ex.FindLabels("bar", []string{"no", "europe", "@"}, []uint16{dns.TypeA})
	if l := len(label.Records[dns.TypeA]); l != 1 {
		t.Logf("Unexpected number of A records: '%d'", l)
		t.Fail()
	}
	if qtype != dns.TypeA {
		t.Fatalf("Expected qtype = A record (type %d), got type %d", dns.TypeA, qtype)
	}
	if str := label.Records[qtype][0].RR.(*dns.A).A.String(); str != "192.168.1.2" {
		t.Logf("Got A '%s', expected '%s'", str, "192.168.1.2")
		t.Fail()
	}

	// label, qtype = ex.FindLabels("", []string{"@"}, []uint16{dns.TypeMX})
	// Mxs := label.Records[dns.TypeMX]
	// c.Check(Mxs, HasLen, 2)
	// c.Check(Mxs[0].RR.(*dns.MX).Mx, Equals, "mx.example.net.")
	// c.Check(Mxs[1].RR.(*dns.MX).Mx, Equals, "mx2.example.net.")

	// label, qtype = ex.FindLabels("", []string{"dk", "europe", "@"}, []uint16{dns.TypeMX})
	// Mxs = label.Records[dns.TypeMX]
	// c.Check(Mxs, HasLen, 1)
	// c.Check(Mxs[0].RR.(*dns.MX).Mx, Equals, "mx-eu.example.net.")
	// c.Check(qtype, Equals, dns.TypeMX)

	// // look for multiple record types
	// label, qtype = ex.FindLabels("www", []string{"@"}, []uint16{dns.TypeCNAME, dns.TypeA})
	// c.Check(label.Records[dns.TypeCNAME], HasLen, 1)
	// c.Check(qtype, Equals, dns.TypeCNAME)

	// // pretty.Println(ex.Labels[""].Records[dns.TypeNS])

	// label, qtype = ex.FindLabels("", []string{"@"}, []uint16{dns.TypeNS})
	// Ns := label.Records[dns.TypeNS]
	// c.Check(Ns, HasLen, 2)
	// // Test that we get the expected NS records (in any order because
	// // of the configuration format used for this zone)
	// c.Check(Ns[0].RR.(*dns.NS).Ns, Matches, "^ns[12]\\.example\\.net.$")
	// c.Check(Ns[1].RR.(*dns.NS).Ns, Matches, "^ns[12]\\.example\\.net.$")

	// label, qtype = ex.FindLabels("", []string{"@"}, []uint16{dns.TypeSPF})
	// Spf := label.Records[dns.TypeSPF]
	// c.Check(Spf, HasLen, 1)
	// c.Check(Spf[0].RR.(*dns.SPF).Txt[0], Equals, "v=spf1 ~all")

	// label, qtype = ex.FindLabels("foo", []string{"@"}, []uint16{dns.TypeTXT})
	// Txt := label.Records[dns.TypeTXT]
	// c.Check(Txt, HasLen, 1)
	// c.Check(Txt[0].RR.(*dns.TXT).Txt[0], Equals, "this is foo")

	// label, qtype = ex.FindLabels("weight", []string{"@"}, []uint16{dns.TypeTXT})
	// Txt = label.Records[dns.TypeTXT]
	// c.Check(Txt, HasLen, 2)
	// c.Check(Txt[0].RR.(*dns.TXT).Txt[0], Equals, "w1000")
	// c.Check(Txt[1].RR.(*dns.TXT).Txt[0], Equals, "w1")

	// //verify empty labels are created
	// label, qtype = ex.FindLabels("a.b.c", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 1)
	// c.Check(label.Records[dns.TypeA][0].RR.(*dns.A).A.String(), Equals, "192.168.1.7")

	// label, qtype = ex.FindLabels("b.c", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 0)
	// c.Check(label.Label, Equals, "b.c")

	// label, qtype = ex.FindLabels("c", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 0)
	// c.Check(label.Label, Equals, "c")

	// //verify label is created
	// label, qtype = ex.FindLabels("three.two.one", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 1)
	// c.Check(label.Records[dns.TypeA][0].RR.(*dns.A).A.String(), Equals, "192.168.1.5")

	// label, qtype = ex.FindLabels("two.one", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 0)
	// c.Check(label.Label, Equals, "two.one")

	// //verify label isn't overwritten
	// label, qtype = ex.FindLabels("one", []string{"@"}, []uint16{dns.TypeA})
	// c.Check(label.Records[dns.TypeA], HasLen, 1)
	// c.Check(label.Records[dns.TypeA][0].RR.(*dns.A).A.String(), Equals, "192.168.1.6")
}

func TestExampleOrgZone(t *testing.T) {
	mm, err := NewMuxManager("../dns", &NilReg{})
	if err != nil {
		t.Fatalf("Loading test zones: %s", err)
	}

	ex, ok := mm.zonelist["test.example.org"]
	if !ok || ex == nil || ex.Labels == nil {
		t.Fatalf("Did not load 'test.example.org' test zone")
	}

	label, qtype := ex.FindLabels("sub", []string{"@"}, []uint16{dns.TypeNS})
	if qtype != dns.TypeNS {
		t.Fatalf("Expected qtype = NS record (type %d), got type %d", dns.TypeNS, qtype)
	}

	Ns := label.Records[qtype]
	if l := len(Ns); l != 2 {
		t.Fatalf("Expected 2 NS records, got '%d'", l)
	}
	// c.Check(Ns[0].RR.(*dns.NS).Ns, Equals, "ns1.example.com.")
	// c.Check(Ns[1].RR.(*dns.NS).Ns, Equals, "ns2.example.com.")

}
