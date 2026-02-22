package server

import (
	"context"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	"github.com/abh/geodns/v3/appconfig"
	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/targeting"
	"github.com/abh/geodns/v3/zones"
)

const (
	PORT = ":8853"
)

func TestServe(t *testing.T) {
	serverInfo := &monitor.ServerInfo{}

	srv := NewServer(appconfig.Config, serverInfo)
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

	t.Run("QueryLog", testQueryLog(srv))

	t.Run("Cname", testCname)
	t.Run("ServingAliases", testServingAliases)
	t.Run("ServingEDNS", testServingEDNS)

	cancel()

	srv.Shutdown()
}

func testServing(t *testing.T) {
	// Query _status on the pgeodns zone
	r := exchange(t, "_status.pgeodns.", dns.TypeTXT)
	require.Len(t, r.Answer, 1, "1 txt record for _status.pgeodns")
	txt := r.Answer[0].(*dns.TXT).TXT.Txt[0]
	if !strings.HasPrefix(txt, "{") {
		t.Log("Unexpected result for _status.pgeodns", txt)
		t.Fail()
	}

	// Allow _country and _status queries as long as the first label is that
	r = exchange(t, "_country.foo.pgeodns.", dns.TypeTXT)
	txt = r.Answer[0].(*dns.TXT).TXT.Txt[0]
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

	// bar is an alias
	r = exchange(t, "bar.test.example.com.", dns.TypeA)
	ip := r.Answer[0].(*dns.A).Addr
	if ip.String() != "192.168.1.2" {
		t.Logf("unexpected A record for bar.test.example.com: %s", ip.String())
		t.Fail()
	}

	// bar is an alias to test, the SOA record should be for test
	r = exchange(t, "_.root-alias.test.example.com.", dns.TypeA)
	if len(r.Answer) > 0 {
		t.Errorf("got answers for _.root-alias.test.example.com")
	}
	if len(r.Ns) == 0 {
		t.Fatalf("_.root-alias.test didn't return auth section")
	}
	if n := r.Ns[0].(*dns.SOA).Header().Name; n != "test.example.com." {
		t.Fatalf("_.root-alias.test didn't have test.example.com soa: %s", n)
	}

	// root-alias is an alias to test (apex), but the NS records shouldn't be on root-alias
	r = exchange(t, "root-alias.test.example.com.", dns.TypeNS)
	if len(r.Answer) > 0 {
		t.Errorf("got unexpected answers for root-alias.test.example.com NS")
	}
	if len(r.Ns) == 0 {
		t.Fatalf("root-alias.test NS didn't return auth section")
	}

	r = exchange(t, "test.example.com.", dns.TypeSOA)
	soa := r.Answer[0].(*dns.SOA)
	serial := soa.SOA.Serial
	assert.Equal(t, 3, int(serial))

	// no AAAA records for 'bar', so check we get a soa record back
	r = exchange(t, "bar.test.example.com.", dns.TypeAAAA)
	soa2 := r.Ns[0].(*dns.SOA)
	if !reflect.DeepEqual(soa, soa2) {
		t.Errorf("AAAA empty NOERROR soa record different from SOA request")
	}

	// CNAMEs
	r = exchange(t, "www.test.example.com.", dns.TypeA)
	// c.Check(r.Answer[0].(*dns.CNAME).Target, Equals, "geo.bitnames.com.")
	if int(r.Answer[0].Header().TTL) != 1800 {
		t.Logf("unexpected ttl '%d' for geo.bitnames.com (expected %d)", int(r.Answer[0].Header().TTL), 1800)
		t.Fail()
	}

	// SPF
	r = exchange(t, "test.example.com.", dns.TypeSPF)
	assert.Equal(t, r.Answer[0].(*dns.SPF).TXT.Txt[0], "v=spf1 ~all")

	// SRV
	r = exchange(t, "_sip._tcp.test.example.com.", dns.TypeSRV)
	assert.Equal(t, r.Answer[0].(*dns.SRV).SRV.Target, "sipserver.example.com.")
	assert.Equal(t, r.Answer[0].(*dns.SRV).SRV.Port, uint16(5060))
	assert.Equal(t, r.Answer[0].(*dns.SRV).SRV.Priority, uint16(10))
	assert.Equal(t, r.Answer[0].(*dns.SRV).SRV.Weight, uint16(100))

	// MX
	r = exchange(t, "test.example.com.", dns.TypeMX)
	assert.Equal(t, r.Answer[0].(*dns.MX).MX.Mx, "mx.example.net.")
	assert.Equal(t, r.Answer[1].(*dns.MX).MX.Mx, "mx2.example.net.")
	assert.Equal(t, r.Answer[1].(*dns.MX).MX.Preference, uint16(20))

	// Verify the first A record was created
	r = exchange(t, "a.b.c.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).Addr
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
	ip = r.Answer[0].(*dns.A).Addr

	assert.Equal(t, ip.String(), "192.168.1.5", "three.two.one.test.example.com A record")

	// Verify single sub-labels is created and no record is returned
	r = exchange(t, "two.one.test.example.com.", dns.TypeA)
	assert.Len(t, r.Answer, 0, "expect 0 answer records for two.one.test.example.com")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "two.one.test.example.com")

	// Verify the A record wasn't over written
	r = exchange(t, "one.test.example.com.", dns.TypeA)
	ip = r.Answer[0].(*dns.A).Addr
	assert.Equal(t, ip.String(), "192.168.1.6", "one.test.example.com A record")

	// PTR
	r = exchange(t, "2.1.168.192.IN-ADDR.ARPA.", dns.TypePTR)
	assert.Len(t, r.Answer, 1, "expect 1 answer records for 2.1.168.192.IN-ADDR.ARPA")
	checkRcode(t, r.Rcode, dns.RcodeSuccess, "2.1.168.192.IN-ADDR.ARPA")

	name := r.Answer[0].(*dns.PTR).PTR.Ptr
	assert.Equal(t, name, "bar.example.com.", "PTR record")
}

func testCname(t *testing.T) {
	// Cname, two possible results
	results := make(map[string]int)

	for range 10 {
		r := exchange(t, "www.se.test.example.com.", dns.TypeA)
		// only return one CNAME even if there are multiple options
		require.Len(t, r.Answer, 1)
		target := r.Answer[0].(*dns.CNAME).CNAME.Target
		results[target]++
	}

	// Two possible results from this cname
	assert.Len(t, results, 2)
}

func testServingAliases(t *testing.T) {
	// Alias, no geo matches
	r := exchange(t, "bar-alias.test.example.com.", dns.TypeA)
	ip := r.Answer[0].(*dns.A).Addr
	assert.Equal(t, "192.168.1.2", ip.String())

	// Alias to a cname record
	r = exchange(t, "www-alias.test.example.com.", dns.TypeA)
	assert.Equal(t, "geo.bitnames.com.", r.Answer[0].(*dns.CNAME).CNAME.Target)

	// Alias returning a cname, with geo overrides (requires GeoIP)
	if targeting.Geo() != nil {
		r = exchangeSubnet(t, "www-alias.test.example.com.", dns.TypeA, "194.239.134.1")
		require.Len(t, r.Answer, 1)
		assert.Equal(t, "geo-europe.bitnames.com.", r.Answer[0].(*dns.CNAME).CNAME.Target)
	}

	// Alias to NS records - aliases intentionally don't follow NS/SOA records (see zone.go)
	// so we expect no answer records, just an authority section
	r = exchange(t, "sub-alias.test.example.org.", dns.TypeNS)
	assert.Len(t, r.Answer, 0, "aliases don't follow NS records")
}

func testServingEDNS(t *testing.T) {
	if targeting.Geo() == nil {
		t.Skip("GeoIP not available")
	}

	// MX test with geo override
	r := exchangeSubnet(t, "test.example.com.", dns.TypeMX, "194.239.134.1")
	require.Len(t, r.Answer, 1)
	assert.Equal(t, "mx-eu.example.net.", r.Answer[0].(*dns.MX).MX.Mx)

	// www.test from .dk IP address gets at least one answer
	t.Log("Testing www.test.example.com from .dk, should match www.europe (a cname)")
	r = exchangeSubnet(t, "www.test.example.com.", dns.TypeA, "194.239.134.0")
	require.Len(t, r.Answer, 1)
	// EDNS-SUBNET test (request A, respond CNAME)
	assert.Equal(t, "geo-europe.bitnames.com.", r.Answer[0].(*dns.CNAME).CNAME.Target)
}

func checkRcode(t *testing.T, rcode uint16, expected uint16, name string) {
	if rcode != expected {
		t.Logf("'%s': rcode!=%s: %s", name, dnsutil.RcodeToString(expected), dnsutil.RcodeToString(rcode))
		t.Fail()
	}
}

func exchangeSubnet(t *testing.T, name string, dnstype uint16, ip string) *dns.Msg {
	msg := new(dns.Msg)

	dnsutil.SetQuestion(msg, name, dnstype)

	o := new(dns.OPT)
	o.Hdr.Name = "."
	e := &dns.SUBNET{
		Scope:   0,
		Address: netip.MustParseAddr(ip),
		Family:  1, // IP4
		Netmask: 32,
	}
	o.Options = append(o.Options, e)
	msg.Pseudo = append(msg.Pseudo, o)

	t.Log("msg", msg)

	return dorequest(t, msg)
}

func exchange(t *testing.T, name string, dnstype uint16) *dns.Msg {
	msg := new(dns.Msg)

	dnsutil.SetQuestion(msg, name, dnstype)
	return dorequest(t, msg)
}

func dorequest(t *testing.T, msg *dns.Msg) *dns.Msg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := dns.Exchange(ctx, msg, "udp", "127.0.0.1"+PORT)
	if err != nil {
		t.Fatalf("request err '%s': %s", msg.String(), err)
	}
	return r
}
