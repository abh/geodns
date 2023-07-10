package server

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/querylog"
	"github.com/abh/geodns/v3/zones"
	"golang.org/x/sync/errgroup"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

type serverMetrics struct {
	Queries *prometheus.CounterVec
}

// Server ...
type Server struct {
	queryLogger        querylog.QueryLogger
	mux                *dns.ServeMux
	PublicDebugQueries bool
	info               *monitor.ServerInfo
	metrics            *serverMetrics

	lock       sync.Mutex
	dnsServers []*dns.Server
}

// NewServer ...
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

// SetQueryLogger configures the query logger. For now it only supports writing to
// a file (and all zones get logged to the same file).
func (srv *Server) SetQueryLogger(logger querylog.QueryLogger) {
	srv.queryLogger = logger
}

// Add adds the Zone to be handled under the specified name
func (srv *Server) Add(name string, zone *zones.Zone) {
	srv.mux.HandleFunc(name, srv.setupServerFunc(zone))
}

// Remove removes the zone name from being handled by the server
func (srv *Server) Remove(name string) {
	srv.mux.HandleRemove(name)
}

func (srv *Server) setupServerFunc(zone *zones.Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		srv.serve(w, r, zone)
	}
}

// ServeDNS calls ServeDNS in the dns package
func (srv *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	srv.mux.ServeDNS(w, r)
}

func (srv *Server) addDNSServer(dnsServer *dns.Server) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	srv.dnsServers = append(srv.dnsServers, dnsServer)
}

// ListenAndServe starts the DNS server on the specified IP
// (both tcp and udp). It returns an error if
// something goes wrong.
func (srv *Server) ListenAndServe(ctx context.Context, ip string) error {

	prots := []string{"udp", "tcp"}

	g, _ := errgroup.WithContext(ctx)

	for _, prot := range prots {

		p := prot

		g.Go(func() error {
			server := &dns.Server{
				Addr:    ip,
				Net:     p,
				Handler: srv,
			}

			srv.addDNSServer(server)

			log.Printf("Opening on %s %s", ip, p)
			if err := server.ListenAndServe(); err != nil {
				log.Printf("geodns: failed to setup %s %s: %s", ip, p, err)
				return err
			}
			return nil
		})
	}

	// the servers will be shutdown when Shutdown() is called
	return g.Wait()
}

// Shutdown gracefully shuts down the server
func (srv *Server) Shutdown() error {
	var errs []error

	for _, dnsServer := range srv.dnsServers {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := dnsServer.ShutdownContext(timeoutCtx)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if srv.queryLogger != nil {
		err := srv.queryLogger.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	err := errors.Join(errs...)

	return err
}
