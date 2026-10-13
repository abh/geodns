// Package edns provides function useful for adding/inspecting OPT records to/in messages.
package edns

import (
	"errors"
	"sync"

	dns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
)

// FindOPT synthesizes an OPT record from the EDNS fields of m. In dns v2 the OPT
// wrapper is never placed in m.Pseudo after Unpack; instead its metadata lands on
// the Msg itself (UDPSize, Security, Version) and each option appears directly in
// m.Pseudo as its concrete EDNS0 type. FindOPT reconstructs a *dns.OPT from those
// fields so callers can inspect and modify options through the familiar interface.
// Returns nil when m carries no EDNS option (UDPSize == 0).
func FindOPT(m *dns.Msg) *dns.OPT {
	if m.UDPSize == 0 {
		return nil
	}
	opt := new(dns.OPT)
	opt.Hdr.Name = "."
	opt.SetUDPSize(m.UDPSize)
	if m.Security {
		opt.SetSecurity(true)
	}
	opt.SetVersion(m.Version)
	for _, rr := range m.Pseudo {
		if e, ok := rr.(dns.EDNS0); ok {
			opt.Options = append(opt.Options, e)
		}
	}
	return opt
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
	if req.UDPSize == 0 {
		return nil, nil
	}
	if req.Version == 0 {
		return nil, nil
	}
	m := new(dns.Msg)
	dnsutil.SetReply(m, req)
	m.Question = nil
	m.UDPSize = dns.MinMsgSize
	m.Rcode = dns.RcodeBadVers
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

// SetSizeAndDo copies EDNS settings from req into m and appends supported options
// from req.Pseudo into m.Pseudo. It returns a synthetic OPT built from m so the
// caller can inspect or modify the copied options (modifications to option fields
// affect the objects now stored in m.Pseudo). Returns nil if req carries no EDNS.
func SetSizeAndDo(req, m *dns.Msg) *dns.OPT {
	if req.UDPSize == 0 {
		return nil
	}
	m.UDPSize = req.UDPSize
	if req.Security {
		m.Security = true
	}
	m.Pseudo = append(m.Pseudo, SupportedOptions(req.Pseudo)...)
	return FindOPT(m)
}

func SupportedOptions(o []dns.RR) []dns.RR {
	supported := make([]dns.RR, 0, 3)
	// For as long as possible try avoid looking up in the map, because that need an Rlock.
	for _, rr := range o {
		e, ok := rr.(dns.EDNS0)
		if !ok {
			continue
		}
		switch code := dns.RRToCode(e); code {
		case dns.CodeNSID:
			fallthrough
		case dns.CodeCOOKIE:
			fallthrough
		case dns.CodeSUBNET:
			supported = append(supported, rr)
		default:
			if SupportedOption(code) {
				supported = append(supported, rr)
			}
		}
	}
	return supported
}
