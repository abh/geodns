package main

import (
	"log"
	"time"
)
import "github.com/miekg/dns"

type Server struct{}

func (srv *Server) setupServerFunc(Zone *Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		serve(w, r, Zone)
	}
}

func (srv *Server) listenAndServe(ip string) {

	prots := []string{"udp", "tcp"}

	for _, prot := range prots {
		go func(p string) {
			server := &dns.Server{Addr: ip, Net: p}

			log.Printf("Opening on %s %s", ip, p)
			if err := server.ListenAndServe(); err != nil {
				log.Fatalf("geodns: failed to setup %s %s: %s", ip, p, err)
			}
			log.Fatalf("geodns: ListenAndServe unexpectedly returned")
		}(prot)
	}
}

func (srv *Server) addHandler(zones Zones, name string, config *Zone) {
	oldZone := zones[name]
	config.SetupMetrics(oldZone)
	zones[name] = config
	dns.HandleFunc(name, srv.setupServerFunc(config))
}

func (srv *Server) zonesReader(dirName string, zones Zones) {
	for {
		srv.zonesReadDir(dirName, zones)
		time.Sleep(5 * time.Second)
	}
}
