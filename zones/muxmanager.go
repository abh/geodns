package zones

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type RegistrationAPI interface {
	Add(string, *Zone)
	Remove(string)
}

type MuxManager struct {
	reg      RegistrationAPI
	zonelist ZoneList
	path     string
	lastRead map[string]*zoneReadRecord
}

type NilReg struct{}

func (r *NilReg) Add(string, *Zone) {}
func (r *NilReg) Remove(string)     {}

// track when each zone was read last
type zoneReadRecord struct {
	time time.Time
	hash string
}

func NewMuxManager(path string, reg RegistrationAPI) (*MuxManager, error) {
	mm := &MuxManager{
		reg:      reg,
		path:     path,
		zonelist: make(ZoneList),
		lastRead: map[string]*zoneReadRecord{},
	}

	mm.setupRootZone()
	mm.setupPgeodnsZone()

	err := mm.reload()

	return mm, err
}

func (mm *MuxManager) Run() {
	for {
		err := mm.reload()
		if err != nil {
			log.Printf("error reading zones: %s", err)
		}
		time.Sleep(2 * time.Second)
	}
}

// GetZones returns the list of currently active zones in the mux manager.
// (todo: rename to Zones() when the Zones struct has been renamed to ZoneList)
func (mm *MuxManager) Zones() ZoneList {
	return mm.zonelist
}

func (mm *MuxManager) reload() error {
	dir, err := ioutil.ReadDir(mm.path)
	if err != nil {
		return fmt.Errorf("could not read '%s': %s", mm.path, err)
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

		zoneName := fileName[0:strings.LastIndex(fileName, ".")]

		seenZones[zoneName] = true

		if _, ok := mm.lastRead[zoneName]; !ok || file.ModTime().After(mm.lastRead[zoneName].time) {
			modTime := file.ModTime()
			if ok {
				log.Printf("Reloading %s\n", fileName)
				mm.lastRead[zoneName].time = modTime
			} else {
				log.Printf("Reading new file %s\n", fileName)
				mm.lastRead[zoneName] = &zoneReadRecord{time: modTime}
			}

			filename := path.Join(mm.path, fileName)

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
			if mm.lastRead[zoneName].hash == sha256 {
				log.Printf("Skipping new file %s as hash is unchanged\n", filename)
				continue
			}

			zone := NewZone(zoneName)
			err := zone.ReadZoneFile(filename)
			if zone == nil || err != nil {
				parseErr = fmt.Errorf("Error reading zone '%s': %s", zoneName, err)
				log.Println(parseErr.Error())
				continue
			}

			(mm.lastRead[zoneName]).hash = sha256

			mm.addHandler(zoneName, zone)
		}
	}

	for zoneName, zone := range mm.zonelist {
		if zoneName == "pgeodns" {
			continue
		}
		if ok, _ := seenZones[zoneName]; ok {
			continue
		}
		log.Println("Removing zone", zone.Origin)
		zone.Close()
		mm.removeHandler(zoneName)
	}

	return parseErr
}

func (mm *MuxManager) addHandler(name string, zone *Zone) {
	oldZone := mm.zonelist[name]
	zone.SetupMetrics(oldZone)
	zone.setupHealthTests()
	mm.zonelist[name] = zone
	mm.reg.Add(name, zone)
}

func (mm *MuxManager) removeHandler(name string) {
	delete(mm.lastRead, name)
	delete(mm.zonelist, name)
	mm.reg.Remove(name)
}

func (mm *MuxManager) setupPgeodnsZone() {
	zoneName := "pgeodns"
	zone := NewZone(zoneName)
	label := new(Label)
	label.Records = make(map[uint16]Records)
	label.Weight = make(map[uint16]int)
	zone.Labels[""] = label
	zone.AddSOA()
	mm.addHandler(zoneName, zone)
}

func (mm *MuxManager) setupRootZone() {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeRefused)
		w.WriteMsg(m)
	})
}

func sha256File(fn string) string {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return ""
	}
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}
