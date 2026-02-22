package server

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	dns "codeberg.org/miekg/dns"
	"github.com/abh/geodns/v3/appconfig"
	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/querylog"
	"github.com/abh/geodns/v3/zones"
	"go.ntppool.org/common/version"
	"golang.org/x/sync/errgroup"

	"github.com/prometheus/client_golang/prometheus"
)

type serverMetrics struct {
	Queries *prometheus.CounterVec
}

// Server ...
type Server struct {
	PublicDebugQueries bool
	DetailedMetrics    bool

	queryLogger querylog.QueryLogger
	mux         *dns.ServeMux
	info        *monitor.ServerInfo
	metrics     *serverMetrics

	lock       sync.Mutex
	dnsServers []*dns.Server
}

// NewServer ...
func NewServer(config *appconfig.AppConfig, si *monitor.ServerInfo) *Server {
	mux := dns.NewServeMux()

	queries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_queries_total",
			Help: "Number of served queries",
		},
		[]string{"zone", "qtype", "qname", "rcode"},
	)
	prometheus.MustRegister(queries)

	version.RegisterMetric("geodns", prometheus.DefaultRegisterer)

	instanceInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "geodns_instance_info",
			Help: "GeoDNS instance information",
		},
		[]string{"ID", "IP", "Group"},
	)
	prometheus.MustRegister(instanceInfo)
	group := ""
	if len(si.Groups) > 0 {
		group = si.Groups[0]
	}
	instanceInfo.WithLabelValues(si.ID, si.IP, group).Set(1)

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

	return &Server{
		PublicDebugQueries: appconfig.Config.DNS.PublicDebugQueries,
		DetailedMetrics:    appconfig.Config.DNS.DetailedMetrics,

		mux:     mux,
		info:    si,
		metrics: metrics,
	}
}

// SetQueryLogger configures the query logger. For now it only supports writing to
// a file (and all zones get logged to the same file).
func (srv *Server) SetQueryLogger(logger querylog.QueryLogger) {
	srv.queryLogger = logger
}

// Add adds the Zone to be handled under the specified name
func (srv *Server) Add(name string, zone *zones.Zone) {
	// v2 ServeMux requires patterns to be in canonical form (FQDN with trailing dot)
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	srv.mux.HandleFunc(name, srv.setupServerFunc(zone))
}

// Remove removes the zone name from being handled by the server
func (srv *Server) Remove(name string) {
	// v2 ServeMux requires patterns to be in canonical form (FQDN with trailing dot)
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	srv.mux.HandleRemove(name)
}

func (srv *Server) setupServerFunc(zone *zones.Zone) func(context.Context, dns.ResponseWriter, *dns.Msg) {
	return func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
		srv.serve(ctx, w, r, zone)
	}
}

// ServeDNS calls ServeDNS in the dns package
func (srv *Server) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	srv.mux.ServeDNS(ctx, w, r)
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
		dnsServer.Shutdown(timeoutCtx)
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
