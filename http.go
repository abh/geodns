package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/abh/geodns/monitor"
	"github.com/abh/geodns/zones"
	metrics "github.com/rcrowley/go-metrics"
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

type histogramData struct {
	Max    int64
	Min    int64
	Mean   float64
	Pct90  float64
	Pct99  float64
	Pct999 float64
	StdDev float64
}

func setupHistogramData(met metrics.Histogram, dat *histogramData) {
	dat.Max = met.Max()
	dat.Min = met.Min()
	dat.Mean = met.Mean()
	dat.StdDev = met.StdDev()
	percentiles := met.Percentiles([]float64{0.90, 0.99, 0.999})
	dat.Pct90 = percentiles[0]
	dat.Pct99 = percentiles[1]
	dat.Pct999 = percentiles[2]
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
	hs.mux.HandleFunc("/status", hs.StatusHandler())
	hs.mux.HandleFunc("/status.json", hs.StatusJSONHandler())
	hs.mux.HandleFunc("/", hs.mainServer)

	return hs
}

func (hs *httpServer) Mux() *http.ServeMux {
	return hs.mux
}

func (hs *httpServer) Run(listen string) {
	log.Println("Starting HTTP interface on", listen)
	log.Fatal(http.ListenAndServe(listen, &basicauth{h: hs.mux}))
}

func (hs *httpServer) StatusJSONHandler() func(http.ResponseWriter, *http.Request) {

	info := serverInfo

	return func(w http.ResponseWriter, req *http.Request) {

		zonemetrics := make(map[string]metrics.Registry)

		for name, zone := range hs.zones.Zones() {
			zone.Lock()
			zonemetrics[name] = zone.Metrics.Registry
			zone.Unlock()
		}

		type statusData struct {
			Version   string
			GoVersion string
			Uptime    int64
			Platform  string
			Zones     map[string]metrics.Registry
			Global    metrics.Registry
			ID        string
			IP        string
			UUID      string
			Groups    []string
		}

		uptime := int64(time.Since(info.Started).Seconds())

		status := statusData{
			Version:   info.Version,
			GoVersion: runtime.Version(),
			Uptime:    uptime,
			Platform:  runtime.GOARCH + "-" + runtime.GOOS,
			Zones:     zonemetrics,
			Global:    metrics.DefaultRegistry,
			ID:        hs.serverInfo.ID,
			IP:        hs.serverInfo.IP,
			UUID:      hs.serverInfo.UUID,
			Groups:    hs.serverInfo.Groups,
		}

		b, err := json.Marshal(status)
		if err != nil {
			http.Error(w, "Error encoding JSON", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}
}

func (hs *httpServer) StatusHandler() func(http.ResponseWriter, *http.Request) {

	type statusData struct {
		Version  string
		Zones    rates
		Uptime   DayDuration
		Platform string
		Global   struct {
			Queries         metrics.Meter
			Histogram       histogramData
			HistogramRecent histogramData
		}
		TopOption int
	}

	return func(w http.ResponseWriter, req *http.Request) {

		topOption := topParam(req, 10)

		rates := make(rates, 0)

		for name, zone := range hs.zones.Zones() {
			count := zone.Metrics.Queries.Count()
			rates = append(rates, &rate{
				Name:    name,
				Count:   count,
				Metrics: zone.Metrics,
			})
		}

		sort.Sort(ratesByCount{rates})

		uptime := DayDuration{time.Since(hs.serverInfo.Started)}

		status := statusData{
			Version:   VERSION,
			Zones:     rates,
			Uptime:    uptime,
			Platform:  runtime.GOARCH + "-" + runtime.GOOS,
			TopOption: topOption,
		}

		status.Global.Queries = metrics.Get("queries").(*metrics.StandardMeter).Snapshot()

		setupHistogramData(metrics.Get("queries-histogram").(*metrics.StandardHistogram).Snapshot(), &status.Global.Histogram)

		statusTemplate, err := FSString(development, "/templates/status.html")
		if err != nil {
			log.Println("Could not read template:", err)
			w.WriteHeader(500)
			return
		}
		tmpl, err := template.New("status_html").Parse(statusTemplate)

		if err != nil {
			str := fmt.Sprintf("Could not parse template: %s", err)
			io.WriteString(w, str)
			return
		}

		err = tmpl.Execute(w, status)
		if err != nil {
			log.Println("Status template error", err)
		}
	}
}

func (hs *httpServer) mainServer(w http.ResponseWriter, req *http.Request) {
	if req.RequestURI != "/version" {
		http.NotFound(w, req)
		return
	}
	io.WriteString(w, `<html><head><title>GeoDNS `+
		hs.serverInfo.Version+`</title><body>`+
		`GeoDNS Server`+
		`</body></html>`)
}

type basicauth struct {
	h http.Handler
}

func (b *basicauth) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// don't request passwords for the websocket interface (for now)
	// because 'wscat' doesn't support that.
	if r.RequestURI == "/monitor" {
		b.h.ServeHTTP(w, r)
		return
	}

	cfgMutex.RLock()
	user := Config.HTTP.User
	password := Config.HTTP.Password
	cfgMutex.RUnlock()

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
	return
}
