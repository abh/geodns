package server

import (
	"log"

	"github.com/abh/geodns/monitor"
	"github.com/abh/geodns/querylog"
	"github.com/abh/geodns/zones"

	"github.com/miekg/dns"
)

type Server struct {
	queryLogger        querylog.QueryLogger
	mux                *dns.ServeMux
	PublicDebugQueries bool
	info               *monitor.ServerInfo
}

func NewServer(si *monitor.ServerInfo) *Server {
	mux := dns.NewServeMux()

	// todo: this should be in the monitor package, or somewhere else.
	// Also if we can stop the server later, need to stop the server too.
	metrics := NewMetrics()
	go metrics.Updater()

	return &Server{mux: mux, info: si}
}

// Setup the QueryLogger. For now it only supports writing to a file (and all
// zones get logged to the same file).
func (srv *Server) SetQueryLogger(logger querylog.QueryLogger) {
	srv.queryLogger = logger
}

func (srv *Server) Add(name string, zone *zones.Zone) {
	srv.mux.HandleFunc(name, srv.setupServerFunc(zone))
}

func (srv *Server) Remove(name string) {
	srv.mux.HandleRemove(name)
}

func (srv *Server) setupServerFunc(zone *zones.Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		srv.serve(w, r, zone)
	}
}

func (srv *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	srv.mux.ServeDNS(w, r)
}

func (srv *Server) ListenAndServe(ip string) {

	prots := []string{"udp", "tcp"}

	for _, prot := range prots {
		go func(p string) {
			server := &dns.Server{
				Addr:    ip,
				Net:     p,
				Handler: srv,
			}

			log.Printf("Opening on %s %s", ip, p)
			if err := server.ListenAndServe(); err != nil {
				log.Fatalf("geodns: failed to setup %s %s: %s", ip, p, err)
			}
			log.Fatalf("geodns: ListenAndServe unexpectedly returned")
		}(prot)
	}
}
