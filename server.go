package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"time"

	"github.com/abh/geodns/applog"
	"github.com/abh/geodns/querylog"
	"github.com/abh/geodns/zones"

	"github.com/miekg/dns"
)

type Server struct {
	queryLogger querylog.QueryLogger
}

// track when each zone was read last
type zoneReadRecord struct {
	time time.Time
	hash string
}

func NewServer() *Server {
	return &Server{}
}

// Setup the QueryLogger. For now it only supports writing to a file (and all
// zones get logged to the same file).
func (srv *Server) SetQueryLogger(logger querylog.QueryLogger) {
	srv.queryLogger = logger
}

func (srv *Server) setupServerFunc(zone *zones.Zone) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		srv.serve(w, r, zone)
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

func (srv *Server) addHandler(zones zones.Zones, name string, config *zones.Zone) {
	oldZone := zones[name]
	// across the recconfiguration keep a reference to all healthchecks to ensure
	// the global map doesn't get destroyed
	// health.TestRunner.refAllGlobalHealthChecks(name, true)
	// defer health.TestRunner.refAllGlobalHealthChecks(name, false)
	// if oldZone != nil {
	// 	oldZone.StartStopHealthChecks(false, nil)
	// }
	config.SetupMetrics(oldZone)
	zones[name] = config
	// config.StartStopHealthChecks(true, oldZone)
	dns.HandleFunc(name, srv.setupServerFunc(config))
}

func (srv *Server) setupPgeodnsZone(zonelist zones.Zones) {
	zoneName := "pgeodns"
	zone := zones.NewZone(zoneName)
	label := new(zones.Label)
	label.Records = make(map[uint16]zones.Records)
	label.Weight = make(map[uint16]int)
	zone.Labels[""] = label
	zone.AddSOA()
	srv.addHandler(zonelist, zoneName, zone)
}

func (srv *Server) setupRootZone() {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeRefused)
		w.WriteMsg(m)
	})
}

var lastRead = map[string]*zoneReadRecord{}

func (srv *Server) zonesReader(dirName string, zones zones.Zones) {
	for {
		err := srv.zonesReadDir(dirName, zones)
		if err != nil {
			log.Printf("error reading zones: %s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (srv *Server) zonesReadDir(dirName string, zonelist zones.Zones) error {
	dir, err := ioutil.ReadDir(dirName)
	if err != nil {
		return fmt.Errorf("could not read", dirName, ":", err)
	}

	seenZones := map[string]bool{}

	var parseErr error

	for _, file := range dir {
		fileName := file.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".json") ||
			strings.HasPrefix(path.Base(fileName), ".") ||
			file.IsDir() {
			continue
		}

		zoneName := zoneNameFromFile(fileName)

		seenZones[zoneName] = true

		if _, ok := lastRead[zoneName]; !ok || file.ModTime().After(lastRead[zoneName].time) {
			modTime := file.ModTime()
			if ok {
				applog.Printf("Reloading %s\n", fileName)
				lastRead[zoneName].time = modTime
			} else {
				applog.Printf("Reading new file %s\n", fileName)
				lastRead[zoneName] = &zoneReadRecord{time: modTime}
			}

			filename := path.Join(dirName, fileName)

			// Check the sha256 of the file has not changed. It's worth an explanation of
			// why there isn't a TOCTOU race here. Conceivably after checking whether the
			// SHA has changed, the contents then change again before we actually load
			// the JSON. This can occur in two situations:
			//
			// 1. The SHA has not changed when we read the file for the SHA, but then
			//    changes before we process the JSON
			//
			// 2. The SHA has changed when we read the file for the SHA, but then changes
			//    again before we process the JSON
			//
			// In circumstance (1) we won't reread the file the first time, but the subsequent
			// change should alter the mtime again, causing us to reread it. This reflects
			// the fact there were actually two changes.
			//
			// In circumstance (2) we have already reread the file once, and then when the
			// contents are changed the mtime changes again
			//
			// Provided files are replaced atomically, this should be OK. If files are not
			// replaced atomically we have other problems (e.g. partial reads).

			sha256 := sha256File(filename)
			if lastRead[zoneName].hash == sha256 {
				applog.Printf("Skipping new file %s as hash is unchanged\n", filename)
				continue
			}

			zone, err := zones.ReadZoneFile(zoneName, filename)
			if zone == nil || err != nil {
				parseErr = fmt.Errorf("Error reading zone '%s': %s", zoneName, err)
				log.Println(parseErr.Error())
				continue
			}

			(lastRead[zoneName]).hash = sha256

			srv.addHandler(zonelist, zoneName, zone)
		}
	}

	for zoneName, zone := range zonelist {
		if zoneName == "pgeodns" {
			continue
		}
		if ok, _ := seenZones[zoneName]; ok {
			continue
		}
		log.Println("Removing zone", zone.Origin)
		delete(lastRead, zoneName)
		zone.Close()
		dns.HandleRemove(zoneName)
		delete(zonelist, zoneName)
	}

	return parseErr
}

func zoneNameFromFile(fileName string) string {
	return fileName[0:strings.LastIndex(fileName, ".")]
}

func sha256File(fn string) string {
	if data, err := ioutil.ReadFile(fn); err != nil {
		return ""
	} else {
		hasher := sha256.New()
		hasher.Write(data)
		return hex.EncodeToString(hasher.Sum(nil))
	}
}
