package main

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

	"code.google.com/p/go.net/websocket"
	"github.com/abh/go-metrics"
)

// Initial status message on websocket
type statusStreamMsgStart struct {
	Hostname string   `json:"h,omitemty"`
	Version  string   `json:"v"`
	ID       string   `json:"id"`
	IP       string   `json:"ip"`
	Uptime   int      `json:"up"`
	Started  int      `json:"started"`
	Groups   []string `json:"groups"`
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

func logStatus() {
	log.Println(initialStatus())

	qCounter := metrics.Get("queries").(metrics.Meter)
	lastQueryCount := qCounter.Count()

	for {
		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		log.Println("goroutines", runtime.NumGoroutine(), "queries", newQueries)

		time.Sleep(60 * time.Second)
	}
}

func monitor(zones Zones) {
	go logStatus()

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

func setupHistogramData(met *metrics.StandardHistogram, dat *histogramData) {
	dat.Max = met.Max()
	dat.Min = met.Min()
	dat.Mean = met.Mean()
	dat.StdDev = met.StdDev()
	percentiles := met.Percentiles([]float64{0.90, 0.99, 0.999})
	dat.Pct90 = percentiles[0]
	dat.Pct99 = percentiles[1]
	dat.Pct999 = percentiles[2]
}

func StatusServer(zones Zones) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {

		req.ParseForm()

		topOption := 10
		topParam := req.Form["top"]

		if len(topParam) > 0 {
			var err error
			topOption, err = strconv.Atoi(topParam[0])
			if err != nil {
				topOption = 10
			}
		}

		statusTemplate, err := templates_status_html()
		if err != nil {
			log.Println("Could not read template", err)
			w.WriteHeader(500)
			return
		}
		tmpl, err := template.New("status_html").Parse(string(statusTemplate))

		if err != nil {
			str := fmt.Sprintf("Could not parse template: %s", err)
			io.WriteString(w, str)
			return
		}

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
				Queries         *metrics.StandardMeter
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

		status.Global.Queries = metrics.Get("queries").(*metrics.StandardMeter)

		setupHistogramData(metrics.Get("queries-histogram").(*metrics.StandardHistogram), &status.Global.Histogram)

		err = tmpl.Execute(w, status)
		if err != nil {
			log.Println("Status template error", err)
		}
	}
}

func httpHandler(zones Zones) {
	http.Handle("/monitor", websocket.Handler(wsHandler))
	http.HandleFunc("/status", StatusServer(zones))
	http.HandleFunc("/", MainServer)

	log.Println("Starting HTTP interface on", *flaghttp)

	log.Fatal(http.ListenAndServe(*flaghttp, nil))
}
