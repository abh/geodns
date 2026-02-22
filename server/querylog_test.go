package server

import (
	"testing"

	"github.com/abh/geodns/v3/querylog"
	dnsv1 "github.com/miekg/dns"
)

type testLogger struct {
	lastLog querylog.Entry
}

func (l *testLogger) Close() error {
	return nil
}

func (l *testLogger) Write(ql *querylog.Entry) error {
	l.lastLog = *ql
	return nil
}

func (l *testLogger) Last() querylog.Entry {
	// l.logged = false
	return l.lastLog
}

func testQueryLog(srv *Server) func(*testing.T) {
	tlog := &testLogger{}

	srv.SetQueryLogger(tlog)

	return func(t *testing.T) {
		r := exchange(t, "www-alias.example.com.", dnsv1.TypeA)
		expected := "geo.bitnames.com."
		answer := r.Answer[0].(*dnsv1.CNAME).Target
		if answer != expected {
			t.Logf("expected CNAME %s, got %s", expected, answer)
			t.Fail()
		}

		last := tlog.Last()
		// t.Logf("last log: %+v", last)

		if last.Name != "www-alias.example.com." {
			t.Logf("didn't get qname in Name querylog")
			t.Fail()
		}
		if last.LabelName != "www" {
			t.Logf("LabelName didn't contain resolved label")
			t.Fail()
		}
	}
}
