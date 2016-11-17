package main

//go:generate esc -o templates.go templates/

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/rcrowley/go-metrics"
	"golang.org/x/net/websocket"
)

// Initial status message on websocket
type statusStreamMsgStart struct {
	Hostname  string   `json:"h,omitemty"`
	Version   string   `json:"v"`
	GoVersion string   `json:"gov"`
	ID        string   `json:"id"`
	IP        string   `json:"ip"`
	UUID      string   `json:"uuid"`
	Uptime    int      `json:"up"`
	Started   int      `json:"started"`
	Groups    []string `json:"groups"`
}

// Update message on websocket
type statusStreamMsgUpdate struct {
	Uptime     int     `json:"up"`
	QueryCount int64   `json:"qs"`
	Qps        int64   `json:"qps"`
	Qps1m      float64 `json:"qps1m,omitempty"`
}

type wsConnection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan string
}

type monitorHub struct {
	connections map[*wsConnection]bool
	broadcast   chan string
	register    chan *wsConnection
	unregister  chan *wsConnection
}

var hub = monitorHub{
	broadcast:   make(chan string),
	register:    make(chan *wsConnection, 10),
	unregister:  make(chan *wsConnection, 10),
	connections: make(map[*wsConnection]bool),
}

func (h *monitorHub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			log.Println("Queuing initial status")
			c.send <- initialStatus()
		case c := <-h.unregister:
			log.Println("Unregistering connection")
			delete(h.connections, c)
		case m := <-h.broadcast:
			for c := range h.connections {
				if len(c.send)+5 > cap(c.send) {
					log.Println("WS connection too close to cap")
					c.send <- `{"error": "too slow"}`
					close(c.send)
					go c.ws.Close()
					h.unregister <- c
					continue
				}
				select {
				case c.send <- m:
				default:
					close(c.send)
					delete(h.connections, c)
					log.Println("Closing channel when sending")
					go c.ws.Close()
				}
			}
		}
	}
}

func (c *wsConnection) reader() {
	for {
		var message string
		err := websocket.Message.Receive(c.ws, &message)
		if err != nil {
			if err == io.EOF {
				log.Println("WS connection closed")
			} else {
				log.Println("WS read error:", err)
			}
			break
		}
		log.Println("WS message", message)
		// TODO(ask) take configuration options etc
		//h.broadcast <- message
	}
	c.ws.Close()
}

func (c *wsConnection) writer() {
	for message := range c.send {
		err := websocket.Message.Send(c.ws, message)
		if err != nil {
			log.Println("WS write error:", err)
			break
		}
	}
	c.ws.Close()
}

func wsHandler(ws *websocket.Conn) {
	log.Println("Starting new WS connection")
	c := &wsConnection{send: make(chan string, 180), ws: ws}
	hub.register <- c
	defer func() {
		log.Println("sending unregister message")
		hub.unregister <- c
	}()
	go c.writer()
	c.reader()
}

func initialStatus() string {
	status := new(statusStreamMsgStart)
	status.Version = VERSION
	status.ID = serverID
	status.IP = serverIP
	status.UUID = serverUUID
	status.GoVersion = runtime.Version()
	if len(serverGroups) > 0 {
		status.Groups = serverGroups
	}
	hostname, err := os.Hostname()
	if err == nil {
		status.Hostname = hostname
	}

	status.Uptime = int(time.Since(timeStarted).Seconds())
	status.Started = int(timeStarted.Unix())

	message, err := json.Marshal(status)
	return string(message)
}

func monitor(zones Zones) {

	if len(*flaghttp) == 0 {
		return
	}
	go hub.run()
	go httpHandler(zones)

	qCounter := metrics.Get("queries").(metrics.Meter)
	lastQueryCount := qCounter.Count()

	status := new(statusStreamMsgUpdate)
	var lastQps1m float64

	for {
		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		status.Uptime = int(time.Since(timeStarted).Seconds())
		status.QueryCount = qCounter.Count()
		status.Qps = newQueries

		newQps1m := qCounter.Rate1()
		if newQps1m != lastQps1m {
			status.Qps1m = newQps1m
			lastQps1m = newQps1m
		} else {
			status.Qps1m = 0
		}

		message, err := json.Marshal(status)

		if err == nil {
			hub.broadcast <- string(message)
		}
		time.Sleep(1 * time.Second)
	}
}

func MainServer(w http.ResponseWriter, req *http.Request) {
	if req.RequestURI != "/version" {
		http.NotFound(w, req)
		return
	}
	io.WriteString(w, `<html><head><title>GeoDNS `+
		VERSION+`</title><body>`+
		initialStatus()+
		`</body></html>`)
}

type rate struct {
	Name    string
	Count   int64
	Metrics ZoneMetrics
}
type Rates []*rate

func (s Rates) Len() int      { return len(s) }
func (s Rates) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type RatesByCount struct{ Rates }

func (s RatesByCount) Less(i, j int) bool {
	ic := s.Rates[i].Count
	jc := s.Rates[j].Count
	if ic == jc {
		return s.Rates[i].Name < s.Rates[j].Name
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

func StatusJSONHandler(zones Zones) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {

		zonemetrics := make(map[string]metrics.Registry)

		for name, zone := range zones {
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

		uptime := int64(time.Since(timeStarted).Seconds())

		status := statusData{
			Version:   VERSION,
			GoVersion: runtime.Version(),
			Uptime:    uptime,
			Platform:  runtime.GOARCH + "-" + runtime.GOOS,
			Zones:     zonemetrics,
			Global:    metrics.DefaultRegistry,
			ID:        serverID,
			IP:        serverIP,
			UUID:      serverUUID,
			Groups:    serverGroups,
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

func StatusHandler(zones Zones) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {

		topOption := topParam(req, 10)

		rates := make(Rates, 0)

		for name, zone := range zones {
			count := zone.Metrics.Queries.Count()
			rates = append(rates, &rate{
				Name:    name,
				Count:   count,
				Metrics: zone.Metrics,
			})
		}

		sort.Sort(RatesByCount{rates})

		type statusData struct {
			Version  string
			Zones    Rates
			Uptime   DayDuration
			Platform string
			Global   struct {
				Queries         metrics.Meter
				Histogram       histogramData
				HistogramRecent histogramData
			}
			TopOption int
		}

		uptime := DayDuration{time.Since(timeStarted)}

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

func httpHandler(zones Zones) {
	http.Handle("/monitor", websocket.Handler(wsHandler))
	http.HandleFunc("/status", StatusHandler(zones))
	http.HandleFunc("/status.json", StatusJSONHandler(zones))
	http.HandleFunc("/", MainServer)

	log.Println("Starting HTTP interface on", *flaghttp)

	log.Fatal(http.ListenAndServe(*flaghttp, &basicauth{h: http.DefaultServeMux}))
}
