package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/abh/geodns/querylog"
	"github.com/miekg/dns"
)

type statsEntry struct {
	Time   int64
	Origin string
	Name   string
	Vendor string
	Label  string
	Qtype  string
	PoolCC string
	Count  int
}

type Stats struct {
	Count int
	Map   map[string]*statsEntry
}

func NewStats() *Stats {
	return &Stats{
		Map: map[string]*statsEntry{},
	}
}

func (s *Stats) Key(e *querylog.Entry) string {
	return fmt.Sprintf("%s %s %s %d", e.Origin, e.Name, e.LabelName, e.Qtype)
}

func vendorName(n string) string {
	idx := strings.Index(n, ".pool.ntp.org.")
	// log.Printf("IDX for %s: %d", n, idx)
	if idx <= 0 {
		return ""
	}
	n = n[0:idx]

	l := dns.SplitDomainName(n)

	v := l[len(l)-1]

	if len(v) == 1 && strings.ContainsAny(v, "01234") {
		return ""
	}

	if len(v) == 2 {
		// country code
		return "_country"
	}

	if v == "asia" || v == "north-america" || v == "europe" || v == "south-america" || v == "oceania" || v == "africa" {
		return "_continent"
	}

	return v
}

func (stats *Stats) Add(e *querylog.Entry) error {
	if e.Rcode > 0 {
		// NXDOMAIN, count separately?
		return nil
	}
	if e.Answers == 0 {
		// No answers, count separately?
		return nil
	}

	var vendor, poolCC string

	if e.Origin == "pool.ntp.org" || strings.HasSuffix(e.Origin, "ntppool.org") {
		vendor = vendorName(e.Name)

		var ok bool
		poolCC, ok = getPoolCC(e.LabelName)
		if !ok {
			log.Printf("Could not get valid poolCC label for %+v", e)
		}
	}

	stats.Count++

	key := stats.Key(e)

	if s, ok := stats.Map[key]; ok {
		s.Count++
	} else {
		stats.Map[key] = &statsEntry{
			// Time:   time.Unix(e.Time/int64(time.Second), 0),
			Time:   e.Time,
			Origin: e.Origin,
			Name:   e.Name,
			Vendor: vendor,
			Label:  e.LabelName,
			PoolCC: poolCC,
			Qtype:  dns.TypeToString[e.Qtype],
			Count:  1,
		}
	}

	return nil
}

func (stats *Stats) Summarize() {
	// pretty.Println(stats)
	var timeStamp int64
	for k := range stats.Map {
		timeStamp = stats.Map[k].Time
		break
	}
	fmt.Printf("Stats %s count total: %d, summarized: %d\n", time.Unix(timeStamp, 0).String(), stats.Count, len(stats.Map))
}
