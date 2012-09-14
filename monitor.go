package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

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
	register:    make(chan *wsConnection),
	unregister:  make(chan *wsConnection),
	connections: make(map[*wsConnection]bool),
}

func (h *monitorHub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			c.send <- initialStatus()
		case c := <-h.unregister:
			delete(h.connections, c)
			//close(c.send) // double close makes us panic
		case m := <-h.broadcast:
			for c := range h.connections {
				if len(c.send)+5 > cap(c.send) {
					log.Println("Too close to cap")
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
			log.Println("WS read error:", err)
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
	status := map[string]string{"v": VERSION, "id": serverId}
	var hostname, err = os.Hostname()
	if err == nil {
		status["h"] = hostname
	}
	message, err := json.Marshal(status)
	return string(message)
}

func logStatus() {
	log.Println(initialStatus())
	lastQueryCount := qCounter

	for {
		newQueries := qCounter - lastQueryCount
		lastQueryCount = qCounter
		log.Println("goroutines", runtime.NumGoroutine(), "queries", newQueries)

		time.Sleep(60 * time.Second)
	}
}

func monitor() {
	go logStatus()

	if len(*flaghttp) == 0 {
		return
	}
	go hub.run()
	go httpHandler()

	lastQueryCount := qCounter
	for {
		newQueries := qCounter - lastQueryCount
		lastQueryCount = qCounter

		status := map[string]string{}
		status["up"] = strconv.Itoa(int(time.Since(timeStarted).Seconds()))
		status["qs"] = strconv.FormatUint(qCounter, 10)
		status["qps"] = strconv.FormatUint(newQueries, 10)

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

func httpHandler() {
	http.Handle("/monitor", websocket.Handler(wsHandler))
	http.HandleFunc("/", MainServer)

	log.Fatal(http.ListenAndServe(*flaghttp, nil))
}
