package monitor

//go:generate esc -o templates.go templates/

import (
	"encoding/json"
	"os"
	"runtime"
	"time"

	"github.com/rcrowley/go-metrics"
	"golang.org/x/net/websocket"
)

type ServerInfo struct {
	Version string
	ID      string
	IP      string
	UUID    string
	Groups  []string
	Started time.Time
}

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

type monitor struct {
	serverInfo *ServerInfo
}

func NewMonitor(serverInfo *ServerInfo) *monitor {
	return &monitor{serverInfo: serverInfo}
}

func (m *monitor) initialStatus() string {
	status := new(statusStreamMsgStart)
	status.Version = m.serverInfo.Version
	status.ID = m.serverInfo.ID
	status.IP = m.serverInfo.IP
	status.UUID = m.serverInfo.UUID

	status.GoVersion = runtime.Version()
	if len(m.serverInfo.Groups) > 0 {
		status.Groups = m.serverInfo.Groups
	}
	hostname, err := os.Hostname()
	if err == nil {
		status.Hostname = hostname
	}

	started := m.serverInfo.Started

	status.Started = int(started.Unix())
	status.Uptime = int(time.Since(started).Seconds())

	message, err := json.Marshal(status)
	return string(message)
}

func (m *monitor) Run() {
	go hub.run(m.initialStatus)

	qCounter := metrics.Get("queries").(metrics.Meter)
	lastQueryCount := qCounter.Count()

	status := new(statusStreamMsgUpdate)
	var lastQps1m float64

	for {
		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		status.Uptime = int(time.Since(m.serverInfo.Started).Seconds())
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

func (m *monitor) Handler() websocket.Handler {
	return websocket.Handler(wsHandler)
}
