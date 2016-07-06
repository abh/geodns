package main

import (
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/stathat/go"
)

func (zs *Zones) statHatPoster() {

	if !Config.HasStatHat() {
		return
	}

	stathatGroups := append(serverGroups, "total", serverID)
	suffix := strings.Join(stathatGroups, ",")

	lastCounts := map[string]int64{}
	lastEdnsCounts := map[string]int64{}

	for name, zone := range *zs {
		lastCounts[name] = zone.Metrics.Queries.Count()
		lastEdnsCounts[name] = zone.Metrics.EdnsQueries.Count()
	}

	for {
		time.Sleep(60 * time.Second)

		for name, zone := range *zs {

			count := zone.Metrics.Queries.Count()
			newCount := count - lastCounts[name]
			lastCounts[name] = count

			if zone.Logging != nil && zone.Logging.StatHat == true {

				apiKey := zone.Logging.StatHatAPI
				if len(apiKey) == 0 {
					apiKey = Config.StatHatApiKey()
				}
				if len(apiKey) == 0 {
					continue
				}
				stathat.PostEZCount("zone "+name+" queries~"+suffix, Config.StatHatApiKey(), int(newCount))

				ednsCount := zone.Metrics.EdnsQueries.Count()
				newEdnsCount := ednsCount - lastEdnsCounts[name]
				lastEdnsCounts[name] = ednsCount
				stathat.PostEZCount("zone "+name+" edns queries~"+suffix, Config.StatHatApiKey(), int(newEdnsCount))

			}
		}
	}
}

func statHatPoster() {

	qCounter := metrics.Get("queries").(metrics.Meter)
	lastQueryCount := qCounter.Count()
	stathatGroups := append(serverGroups, "total", serverID)
	suffix := strings.Join(stathatGroups, ",")
	// stathat.Verbose = true

	for {
		time.Sleep(60 * time.Second)

		if !Config.HasStatHat() {
			log.Println("No stathat configuration")
			continue
		}

		log.Println("Posting to stathat")

		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		stathat.PostEZCount("queries~"+suffix, Config.StatHatApiKey(), int(newQueries))
		stathat.PostEZValue("goroutines "+serverID, Config.StatHatApiKey(), float64(runtime.NumGoroutine()))

	}
}
