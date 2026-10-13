package edns

import (
	"net/netip"
	"testing"

	dns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindOPT(t *testing.T) {
	// Build a request the way a client does: put ECS into msg.Pseudo.
	out := new(dns.Msg)
	dnsutil.SetQuestion(out, "example.com.", dns.TypeA)
	out.UDPSize = 4096
	ecs := &dns.SUBNET{
		Family:  1,
		Netmask: 32,
		Scope:   0,
		Address: netip.MustParseAddr("1.2.3.4"),
	}
	out.Pseudo = []dns.RR{ecs}
	require.NoError(t, out.Pack())

	// Simulate a message arriving at the handler: Data holds the full wire bytes,
	// Options is set to MsgOptionUnpack for a complete parse.
	in := new(dns.Msg)
	in.Data = out.Data
	in.Options = dns.MsgOptionUnpack
	require.NoError(t, in.Unpack())

	opt := FindOPT(in)
	require.NotNil(t, opt, "FindOPT must return an OPT record when EDNS is present")
	assert.Equal(t, netip.MustParseAddr("1.2.3.4"), opt.Options[0].(*dns.SUBNET).Address)
}
