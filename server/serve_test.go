package server

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/zones"
	"github.com/miekg/dns"
)

const (
	PORT = ":8853"
)

func TestServe(t *testing.T) {
	serverInfo := &monitor.ServerInfo{}

	srv := NewServer(serverInfo)
	ctx, cancel := context.WithCancel(context.Background())

	mm, err := zones.NewMuxManager("../dns", srv)
	if err != nil {
		t.Fatalf("Loading test zones: %s", err)
	}
	go mm.Run(ctx)

	go func() {
		srv.ListenAndServe(ctx, PORT)
	}()

	// ensure service has properly started before we query it
	time.Sleep(500 * time.Millisecond)

	t.Run("Serving", testServing)

	// todo: run test queries?

	cancel()

	srv.Shutdown()
}

func testServing(t *testing.T) {
	r := exchange(t, "_status.pgeodns.", dns.TypeTXT)
	require.Len(t, r.Answer, 1, "1 txt record for _status.pgeodns")
	txt := r.Answer[0].(*dns.TXT).Txt[0]
	if !strings.HasPrefix(txt, "{") {
		t.Log("Unexpected result for _status.pgeodns", txt)
		t.Fail()
	}

	// Allow _country and _status queries as long as the first label is that
	r = exchange(t, "_country.foo.pgeodns.", dns.TypeTXT)
	txt = r.Answer[0].(*dns.TXT).Txt[0]
	// Got appropriate response for _country txt query
	if !strings.HasPrefix(txt, "127.0.0.1:") {
		t.Log("Unexpected result for _country.foo.pgeodns", txt)
		t.Fail()
	}

	// Make sure A requests for _status doesn't NXDOMAIN
	r = exchange(t, "_status.pgeodns.", dns.TypeA)
	if len(r.Answer) != 0 {
		t.Log("got A record for _status.pgeodns")
		t.Fail()
	}
	if len(r.Ns) != 1 {
		t.Logf("Expected 1 SOA record, got %d", len(r.Ns))
		t.Fail()
	}
	// NOERROR for A request
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "_status.pgeodns")

	r = exchange(t, "bar.test.example.com.", dns.TypeA)
	ip := r.Answer[0].(*dns.A).A

	// c.Check(ip.String(), Equals, "192.168.1.2")
	// c.Check(int(r.Answer[0].Header().Ttl), Equals, 601)

	r = exchange(t, "test.example.com.", dns.TypeSOA)
	soa := r.Answer[0].(*dns.SOA)
	serial := soa.Serial
	assert.Equal(t, 3, int(serial))

	// no AAAA records for 'bar', so check we get a soa record back
	r = exchange(t, "test.example.com.", dns.TypeAAAA)
	soa2 := r.Ns[0].(*dns.SOA)
	if !reflect.DeepEqual(soa, soa2) {
		t.Logf("AAAA empty NOERROR soa record different from SOA request")
		t.Fail()
	}

	// CNAMEs
	r = exchange(t, "www.test.example.com.", dns.TypeA)
	// c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")
	if int(r.Answer[0].Header().Ttl) != 1800 {
		t.Logf("unexpected ttl '%d' for geo.bitnames.com (expected %d)", int(r.Answer[0].Header().Ttl), 1800)
		t.Fail()
	}

	//SPF
	r = exchange(t, "test.example.com.", dns.TypeSPF)
	assert.Equal(t, r.Answer[0].(*dns.SPF).Txt[0], "v=spf1 ~all")

	//SRV
	r = exchange(t, "_sip._tcp.test.example.com.", dns.TypeSRV)
	assert.Equal(t, r.Answer[0].(*dns.SRV).Target, "sipserver.example.com.")
	assert.Equal(t, r.Answer[0].(*dns.SRV).Port, uint16(5060))
	assert.Equal(t, r.Answer[0].(*dns.SRV).Priority, uint16(10))
	assert.Equal(t, r.Answer[0].(*dns.SRV).Weight, uint16(100))

	// MX
	r = exchange(t, "test.example.com.", dns.TypeMX)
	assert.Equal(t, r.Answer[0].(*dns.MX).Mx, "mx.example.net.")
	assert.Equal(t, r.Answer[1].(*dns.MX).Mx, "mx2.example.net.")
	assert.Equal(t, r.Answer[1].(*dns.MX).Preference, uint16(20))

	// Verify the first A record was created
	r = exchange(t, "a.b.c.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A
	assert.Equal(t, ip.String(), "192.168.1.7")

	// Verify sub-labels are created
	r = exchange(t, "b.c.test.example.com.", dns.TypeA)
	assert.Len(t, r.Answer, 0, "expect 0 answer records for b.c.test.example.com")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "b.c.test.example.com")

	r = exchange(t, "c.test.example.com.", dns.TypeA)
	assert.Len(t, r.Answer, 0, "expect 0 answer records for c.test.example.com")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "c.test.example.com")

	// Verify the first A record was created
	r = exchange(t, "three.two.one.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A

	assert.Equal(t, ip.String(), "192.168.1.5", "three.two.one.test.example.com A record")

	// Verify single sub-labels is created and no record is returned
	r = exchange(t, "two.one.test.example.com.", dns.TypeA)
	assert.Len(t, r.Answer, 0, "expect 0 answer records for two.one.test.example.com")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "two.one.test.example.com")

	// Verify the A record wasn't over written
	r = exchange(t, "one.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).A
	assert.Equal(t, ip.String(), "192.168.1.6", "one.test.example.com A record")

	// PTR
	r = exchange(t, "2.1.168.192.IN-ADDR.ARPA.", dns.TypePTR)
	assert.Len(t, r.Answer, 1, "expect 1 answer records for 2.1.168.192.IN-ADDR.ARPA")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "2.1.168.192.IN-ADDR.ARPA")

	name := r.Answer[0].(*dns.PTR).Ptr
	assert.Equal(t, name, "bar.example.com.", "PTR record")
}

// func TestServingMixedCase(t *testing.T) {

// 	r := exchange(c, "_sTaTUs.pGEOdns.", dns.TypeTXT)
// 	checkRcode(t, r.Rcode, dns.RcodeSuccess, "_sTaTUs.pGEOdns.")

// 	txt := r.Answer[0].(*dns.TXT).Txt[0]
// 	if !strings.HasPrefix(txt, "{") {
// 		t.Log("Unexpected result for _status.pgeodns", txt)
// 		t.Fail()
// 	}

// 	n := "baR.test.eXAmPLe.cOM."
// 	r = exchange(c, n, dns.TypeA)
// 	ip := r.Answer[0].(*dns.A).A
// 	c.Check(ip.String(), Equals, "192.168.1.2")
// 	c.Check(r.Answer[0].Header().Name, Equals, n)

// }

// func TestCname(t *testing.T) {
// 	// Cname, two possible results

// 	results := make(map[string]int)

// 	for i := 0; i < 10; i++ {
// 		r := exchange(c, "www.se.test.example.com.", dns.TypeA)
// 		// only return one CNAME even if there are multiple options
// 		c.Check(r.Answer, HasLen, 1)
// 		target := r.Answer[0].(*dns.CNAME).Target
// 		results[target]++
// 	}

// 	// Two possible results from this cname
// 	c.Check(results, HasLen, 2)
// }

// func testUnknownDomain(t *testing.T) {
// 	r := exchange(t, "no.such.domain.", dns.TypeAAAA)
// 	c.Assert(r.Rcode, Equals, dns.RcodeRefused)
// }

// func testServingAliases(t *testing.T) {
// 	// Alias, no geo matches
// 	r := exchange(c, "bar-alias.test.example.com.", dns.TypeA)
// 	ip := r.Answer[0].(*dns.A).A
// 	c.Check(ip.String(), Equals, "192.168.1.2")

// 	// Alias to a cname record
// 	r = exchange(c, "www-alias.test.example.com.", dns.TypeA)
// 	c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")

// 	// Alias returning a cname, with geo overrides
// 	r = exchangeSubnet(c, "www-alias.test.example.com.", dns.TypeA, "194.239.134.1")
// 	c.Check(r.Answer, HasLen, 1)
// 	if len(r.Answer) > 0 {
// 		c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo-europe.bitnames.com.")
// 	}

// 	// Alias to Ns records
// 	r = exchange(c, "sub-alias.test.example.org.", dns.TypeNS)
// 	c.Check(r.Answer[0].(*dns.NS).Ns, Equals, "ns1.example.com.")

// }

// func testServingEDNS(t *testing.T) {
// 	// MX test
// 	r := exchangeSubnet(t, "test.example.com.", dns.TypeMX, "194.239.134.1")
// 	c.Check(r.Answer, HasLen, 1)
// 	if len(r.Answer) > 0 {
// 		c.Check(r.Answer[0].(*dns.MX).Mx, Equals, "mx-eu.example.net.")
// 	}

// 	c.Log("Testing www.test.example.com from .dk, should match www.europe (a cname)")

// 	r = exchangeSubnet(c, "www.test.example.com.", dns.TypeA, "194.239.134.0")
// 	// www.test from .dk IP address gets at least one answer
// 	c.Check(r.Answer, HasLen, 1)
// 	if len(r.Answer) > 0 {
// 		// EDNS-SUBNET test (request A, respond CNAME)
// 		c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo-europe.bitnames.com.")
// 	}

// }

// func TestServeRace(t *testing.T) {
// 	wg := sync.WaitGroup{}
// 	for i := 0; i < 5; i++ {
// 		wg.Add(1)
// 		go func() {
// 			s.TestServing(t)
// 			wg.Done()
// 		}()
// 	}
// 	wg.Wait()
// }

// func BenchmarkServingCountryDebug(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		exchange(b, "_country.foo.pgeodns.", dns.TypeTXT)
// 	}
// }

// func BenchmarkServing(b *testing.B) {

// 	// a deterministic seed is the default anyway, but let's be explicit we want it here.
// 	rnd := rand.NewSource(1)

// 	testNames := []string{"foo.test.example.com.", "one.test.example.com.",
// 		"weight.test.example.com.", "three.two.one.test.example.com.",
// 		"bar.test.example.com.", "0-alias.test.example.com.",
// 	}

// 	for i := 0; i < c.N; i++ {
// 		name := testNames[rnd.Int63()%int64(len(testNames))]
// 		exchange(t, name, dns.TypeA)
// 	}
// }

func checkRcode(t *testing.T, rcode int, expected int, name string) {
	if rcode != expected {
		t.Logf("'%s': rcode!=%s: %s", name, dns.RcodeToString[expected], dns.RcodeToString[rcode])
		t.Fail()
	}
}

func exchangeSubnet(t *testing.T, name string, dnstype uint16, ip string) *dns.Msg {
	msg := new(dns.Msg)

	msg.SetQuestion(name, dnstype)

	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	e := new(dns.EDNS0_SUBNET)
	e.Code = dns.EDNS0SUBNET
	e.SourceScope = 0
	e.Address = net.ParseIP(ip)
	e.Family = 1 // IP4
	e.SourceNetmask = net.IPv4len * 8
	o.Option = append(o.Option, e)
	msg.Extra = append(msg.Extra, o)

	t.Log("msg", msg)

	return dorequest(t, msg)
}

func exchange(t *testing.T, name string, dnstype uint16) *dns.Msg {
	msg := new(dns.Msg)

	msg.SetQuestion(name, dnstype)
	return dorequest(t, msg)
}

func dorequest(t *testing.T, msg *dns.Msg) *dns.Msg {
	cli := new(dns.Client)
	// cli.ReadTimeout = 2 * time.Second
	r, _, err := cli.Exchange(msg, "127.0.0.1"+PORT)
	if err != nil {
		t.Logf("request err '%s': %s", msg.String(), err)
		t.Fail()
	}
	return r
}
