package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/abh/geodns/v3/appconfig"
	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/zones"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

type httpServer struct {
	mux        *http.ServeMux
	zones      *zones.MuxManager
	serverInfo *monitor.ServerInfo
}

type rate struct {
	Name    string
	Count   int64
	Metrics zones.ZoneMetrics
}
type rates []*rate

func (s rates) Len() int      { return len(s) }
func (s rates) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ratesByCount struct{ rates }

func (s ratesByCount) Less(i, j int) bool {
	ic := s.rates[i].Count
	jc := s.rates[j].Count
	if ic == jc {
		return s.rates[i].Name < s.rates[j].Name
	}
	return ic > jc
}

func topParam(req *http.Request, def int) int {
	req.ParseForm()

	topOption := def
	topParam := req.Form["top"]

	if len(topParam) > 0 {
		var err error
		topOption, err = strconv.Atoi(topParam[0])
		if err != nil {
			topOption = def
		}
	}

	return topOption
}

func NewHTTPServer(mm *zones.MuxManager, serverInfo *monitor.ServerInfo) *httpServer {

	hs := &httpServer{
		zones:      mm,
		mux:        &http.ServeMux{},
		serverInfo: serverInfo,
	}
	hs.mux.HandleFunc("/", hs.mainServer)
	hs.mux.Handle("/metrics", promhttp.Handler())

	return hs
}

func (hs *httpServer) Mux() *http.ServeMux {
	return hs.mux
}

func (hs *httpServer) Run(ctx context.Context, listen string) error {
	log.Println("Starting HTTP interface on", listen)

	srv := http.Server{
		Addr:         listen,
		Handler:      &basicauth{h: hs.mux},
		ReadTimeout:  5 * time.Second,
		IdleTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := srv.ListenAndServe()
		if err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		log.Printf("shutting down http server")
		return srv.Shutdown(ctx)
	})

	return g.Wait()
}

func (hs *httpServer) mainServer(w http.ResponseWriter, req *http.Request) {
	if req.RequestURI != "/version" {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)
	io.WriteString(w, `GeoDNS `+hs.serverInfo.Version+`\n`)
}

type basicauth struct {
	h http.Handler
}

func (b *basicauth) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// cfgMutex.RLock()
	user := appconfig.Config.HTTP.User
	password := appconfig.Config.HTTP.Password
	// cfgMutex.RUnlock()

	if len(user) == 0 {
		b.h.ServeHTTP(w, r)
		return
	}

	ruser, rpass, ok := r.BasicAuth()
	if ok {
		if ruser == user && rpass == password {
			b.h.ServeHTTP(w, r)
			return
		}
	}

	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q`, "GeoDNS Status"))
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
