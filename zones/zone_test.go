package zones

import (
	"regexp"
	"testing"

	dns "codeberg.org/miekg/dns"
)

func TestExampleComZone(t *testing.T) {
	t.Log("example com")
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
	m := ex.findFirstLabel("bar", []string{"no", "europe", "@"}, []uint16{dns.TypeA})
	if l := len(m.Label.Records[dns.TypeA]); l != 1 {
		t.Logf("Unexpected number of A records: '%d'", l)
		t.Fail()
	}
	if m.Type != dns.TypeA {
		t.Fatalf("Expected qtype = A record (type %d), got type %d", dns.TypeA, m.Type)
	}
	if str := m.Label.Records[m.Type][0].RR.(*dns.A).Addr.String(); str != "192.168.1.2" {
		t.Errorf("Got A '%s', expected '%s'", str, "192.168.1.2")
	}

	m = ex.findFirstLabel("", []string{"@"}, []uint16{dns.TypeMX})

	Mx := m.Label.Records[dns.TypeMX]
	if len(Mx) != 2 {
		t.Errorf("Expected 2 MX records but got %d", len(Mx))
	}
	if Mx[0].RR.(*dns.MX).MX.Mx != "mx.example.net." {
		t.Errorf("First MX should have been mx.example.net, but was %s", Mx[0].RR.(*dns.MX).MX.Mx)
	}

	m = ex.findFirstLabel("", []string{"dk", "europe", "@"}, []uint16{dns.TypeMX})
	Mx = m.Label.Records[dns.TypeMX]
	if len(Mx) != 1 {
		t.Errorf("Got %d MX record for dk,europe,@ - expected %d", len(Mx), 1)
	}
	if Mx[0].RR.(*dns.MX).MX.Mx != "mx-eu.example.net." {
		t.Errorf("First MX should have been mx-eu.example.net, but was %s", Mx[0].RR.(*dns.MX).MX.Mx)
	}

	// // look for multiple record types
	m = ex.findFirstLabel("www", []string{"@"}, []uint16{dns.TypeCNAME, dns.TypeA})
	if m.Type != dns.TypeCNAME {
		t.Errorf("www should have been a CNAME, but was a %s", dns.TypeToString[m.Type])
	}

	m = ex.findFirstLabel("", []string{"@"}, []uint16{dns.TypeNS})
	Ns := m.Label.Records[dns.TypeNS]
	if len(Ns) != 2 {
		t.Errorf("root should have returned 2 NS records but got %d", len(Ns))
	}

	// Test that we get the expected NS records (in any order because
	// of the configuration format used for this zone)
	re := regexp.MustCompile(`^ns[12]\.example\.net.$`)
	for i := 0; i < 2; i++ {
		if matched := re.MatchString(Ns[i].RR.(*dns.NS).Ns); !matched {
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("Unexpected NS record data '%s'", Ns[i].RR.(*dns.NS).Ns)
		}
	}

	m = ex.findFirstLabel("", []string{"@"}, []uint16{dns.TypeSPF})
	Spf := m.Label.Records[dns.TypeSPF]
	if txt := Spf[0].RR.(*dns.SPF).Txt[0]; txt != "v=spf1 ~all" {
		t.Errorf("Wrong SPF data '%s'", txt)
	}

	m = ex.findFirstLabel("foo", []string{"@"}, []uint16{dns.TypeTXT})
	Txt := m.Label.Records[dns.TypeTXT]
	if txt := Txt[0].RR.(*dns.TXT).Txt[0]; txt != "this is foo" {
		t.Errorf("Wrong TXT data '%s'", txt)
	}

	m = ex.findFirstLabel("weight", []string{"@"}, []uint16{dns.TypeTXT})
	Txt = m.Label.Records[dns.TypeTXT]

	txts := []string{"w10000", "w1"}
	for i, r := range Txt {
		if txt := r.RR.(*dns.TXT).Txt[0]; txt != txts[i] {
			t.Errorf("txt record %d was '%s', expected '%s'", i, txt, txts[i])
		}
	}

	// verify empty labels are created
	m = ex.findFirstLabel("a.b.c", []string{"@"}, []uint16{dns.TypeA})
	if a := m.Label.Records[dns.TypeA][0].RR.(*dns.A); a.Addr.String() != "192.168.1.7" {
		t.Errorf("unexpected IP for a.b.c '%s'", a)
	}

	emptyLabels := []string{"b.c", "c"}
	for _, el := range emptyLabels {
		m = ex.findFirstLabel(el, []string{"@"}, []uint16{dns.TypeA})
		if len(m.Label.Records[dns.TypeA]) > 0 {
			t.Errorf("Unexpected A record for '%s'", el)
		}
		if m.Label.Label != el {
			t.Errorf("'%s' label is '%s'", el, m.Label.Label)
		}
	}

	// verify label is created
	m = ex.findFirstLabel("three.two.one", []string{"@"}, []uint16{dns.TypeA})
	if l := len(m.Label.Records[dns.TypeA]); l != 1 {
		t.Errorf("Unexpected A record count for 'three.two.one' %d, expected 1", l)
	}
	if a := m.Label.Records[dns.TypeA][0].RR.(*dns.A); a.Addr.String() != "192.168.1.5" {
		t.Errorf("unexpected IP for three.two.one '%s'", a)
	}

	el := "two.one"
	m = ex.findFirstLabel(el, []string{"@"}, []uint16{dns.TypeA})
	if len(m.Label.Records[dns.TypeA]) > 0 {
		t.Errorf("Unexpected A record for '%s'", el)
	}
	if m.Label.Label != el {
		t.Errorf("'%s' label is '%s'", el, m.Label.Label)
	}

	// verify label isn't overwritten
	m = ex.findFirstLabel("one", []string{"@"}, []uint16{dns.TypeA})
	if l := len(m.Label.Records[dns.TypeA]); l != 1 {
		t.Errorf("Unexpected A record count for 'one' %d, expected 1", l)
	}
	if a := m.Label.Records[dns.TypeA][0].RR.(*dns.A); a.Addr.String() != "192.168.1.6" {
		t.Errorf("unexpected IP for one '%s'", a)
	}
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

	matches := ex.FindLabels("sub", []string{"@"}, []uint16{dns.TypeNS})
	if matches[0].Type != dns.TypeNS {
		t.Fatalf("Expected qtype = NS record (type %d), got type %d", dns.TypeNS, matches[0].Type)
	}

	Ns := matches[0].Label.Records[matches[0].Type]
	if l := len(Ns); l != 2 {
		t.Fatalf("Expected 2 NS records, got '%d'", l)
	}
}
