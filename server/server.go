package server

import (
	"log"

	"github.com/abh/geodns/monitor"
	"github.com/abh/geodns/querylog"
	"github.com/abh/geodns/zones"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

type serverMetrics struct {
	Queries *prometheus.CounterVec
}

type Server struct {
	queryLogger        querylog.QueryLogger
	mux                *dns.ServeMux
	PublicDebugQueries bool
	info               *monitor.ServerInfo
	metrics            *serverMetrics
}

func NewServer(si *monitor.ServerInfo) *Server {
	mux := dns.NewServeMux()

	queries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_queries_total",
			Help: "Number of served queries",
		},
		[]string{"zone", "qtype", "qname", "rcode"},
	)
	prometheus.MustRegister(queries)

	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "geodns_build_info",
			Help: "GeoDNS build information (in labels)",
		},
		[]string{"Version", "ID", "IP", "Group"},
	)
	prometheus.MustRegister(buildInfo)

	group := ""
	if len(si.Groups) > 0 {
		group = si.Groups[0]
	}
	buildInfo.WithLabelValues(si.Version, si.ID, si.IP, group).Set(1)

	startTime := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "geodns_start_time_seconds",
			Help: "Unix time process started",
		},
	)
	prometheus.MustRegister(startTime)

	nano := si.Started.UnixNano()
	startTime.Set(float64(nano) / 1e9)

	metrics := &serverMetrics{
		Queries: queries,
	}

	return &Server{mux: mux, info: si, metrics: metrics}
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
