// Package edns provides function useful for adding/inspecting OPT records to/in messages.
package edns

import (
	"errors"
	"sync"

	dns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

// findOPT returns the OPT record from the Pseudo section of the message, or nil if not found.
// This replaces the v1 IsEdns0() method.
func findOPT(m *dns.Msg) *dns.OPT {
	for _, rr := range m.Pseudo {
		if opt, ok := rr.(*dns.OPT); ok {
			return opt
		}
	}
	return nil
}

var sup = &supported{m: make(map[uint16]struct{})}

type supported struct {
	m map[uint16]struct{}
	sync.RWMutex
}

// SetSupportedOption adds a new supported option the set of EDNS0 options that we support. Plugins typically call
// this in their setup code to signal support for a new option.
// By default we support:
// dns.CodeNSID, dns.CodeEXPIRE, dns.CodeCOOKIE, dns.CodeTCPKEEPALIVE, dns.CodePADDING. These
// values are not in this map and checked directly in the server.
func SetSupportedOption(option uint16) {
	sup.Lock()
	sup.m[option] = struct{}{}
	sup.Unlock()
}

// SupportedOption returns true if the option code is supported as an extra EDNS0 option.
func SupportedOption(option uint16) bool {
	sup.RLock()
	_, ok := sup.m[option]
	sup.RUnlock()
	return ok
}

// Version checks the EDNS version in the request. If error
// is nil everything is OK and we can invoke the plugin. If non-nil, the
// returned Msg is valid to be returned to the client (and should). For some
// reason this response should not contain a question RR in the question section.
func Version(req *dns.Msg) (*dns.Msg, error) {
	opt := findOPT(req)
	if opt == nil {
		return nil, nil
	}
	if opt.Version() == 0 {
		return nil, nil
	}
	m := new(dns.Msg)
	dnsutil.SetReply(m, req)
	// zero out question section, wtf.
	m.Question = nil

	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.SetVersion(0)
	m.Rcode = dns.RcodeBadVers
	o.SetRcode(dns.RcodeBadVers)
	m.Pseudo = []dns.RR{o}

	return m, errors.New("EDNS0 BADVERS")
}

// Size returns a normalized size based on proto.
func Size(proto string, size uint16) uint16 {
	if proto == "tcp" {
		return dns.MaxMsgSize
	}
	if size < dns.MinMsgSize {
		return dns.MinMsgSize
	}
	return size
}

/*

The below wasn't from the edns package

*/

// SetSizeAndDo adds an OPT record that the reflects the intent from request.
func SetSizeAndDo(req, m *dns.Msg) *dns.OPT {
	o := findOPT(req)
	if o == nil {
		return nil
	}

	if mo := findOPT(m); mo != nil {
		mo.Hdr.Name = "."
		mo.SetVersion(0)
		mo.SetUDPSize(o.UDPSize())
		mo.SetZ(0)            // clear Z flags
		mo.SetSecurity(false) // clear DO bit

		// Assume if the message m has options set, they are OK and represent what an upstream can do.

		if o.Security() {
			mo.SetSecurity(true)
		}
		return mo
	}

	// Reuse the request's OPT record and tack it to m.
	o.Hdr.Name = "."
	o.SetVersion(0)
	o.SetZ(0)            // clear Z flags
	o.SetSecurity(false) // clear DO bit

	if len(o.Options) > 0 {
		o.Options = SupportedOptions(o.Options)
	}

	m.Pseudo = append(m.Pseudo, o)
	return o
}

func SupportedOptions(o []dns.EDNS0) []dns.EDNS0 {
	supported := make([]dns.EDNS0, 0, 3)
	// For as long as possible try avoid looking up in the map, because that need an Rlock.
	for _, opt := range o {
		switch code := dns.RRToCode(opt); code {
		case dns.CodeNSID:
			fallthrough
		case dns.CodeCOOKIE:
			fallthrough
		case dns.CodeSUBNET:
			supported = append(supported, opt)
		default:
			if SupportedOption(code) {
				supported = append(supported, opt)
			}
		}
	}
	return supported
}
