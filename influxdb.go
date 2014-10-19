package main

import (
	"log"
	"time"

	"github.com/influxdb/influxdb-go"
	"github.com/kr/pretty"
)

func (zs *Zones) influxdbPoster() {

	log.Println("starting influxdb poster")

	if len(Config.InfluxDB.Database) == 0 {
		return
	}

	log.Println("going to post to influxdb")

	influxGroups := append(serverGroups, "total", serverID)
	lastCounts := map[string]int64{}
	lastEdnsCounts := map[string]int64{}

	for name, zone := range *zs {
		if zone.Logging.InfluxDB == true {
			lastCounts[name] = zone.Metrics.Queries.Count()
			lastEdnsCounts[name] = zone.Metrics.EdnsQueries.Count()
		}
	}

	influxConfig := &influxdb.ClientConfig{
		Host:     "localhost:8086",
		Username: "geodns",
		Password: "foobartty",
		Database: "geodns",
	}

	inflx, err := influxdb.NewClient(influxConfig)
	if err != nil {
		log.Fatal(err)
	}

	for {
		time.Sleep(60 * time.Second)

		for name, zone := range *zs {

			count := zone.Metrics.Queries.Count()
			newCount := count - lastCounts[name]
			lastCounts[name] = count

			ednsCount := zone.Metrics.EdnsQueries.Count()
			newEdnsCount := ednsCount - lastEdnsCounts[name]
			lastEdnsCounts[name] = ednsCount

			if zone.Logging != nil && zone.Logging.StatHat == true {

				apiKey := zone.Logging.StatHatAPI
				if len(apiKey) == 0 {
					apiKey = Config.StatHat.ApiKey
				}
				if len(apiKey) == 0 {
					continue
				}

				srs := make([]*influxdb.Series, 0)
				cnt := int(newCount)
				ednsCnt := int(newEdnsCount)

				for _, group := range influxGroups {

					name := zone.Origin + "-queries-" + group

					s := &influxdb.Series{
						Name:    name,
						Columns: []string{"queries", "edns-queries"},
						Points: [][]interface{}{
							[]interface{}{cnt, ednsCnt},
						},
					}
					srs = append(srs, s)
				}

				pretty.Println("influx series", srs)

				err := inflx.WriteSeries(srs)
				if err != nil {
					log.Printf("Could not write to influxdb: %s", err)
				}
			}
		}
	}
}

func influxdbPoster() {

	// lastQueryCount := qCounter.Count()
	// stathatGroups := append(serverGroups, "total", serverID)
	// suffix := strings.Join(stathatGroups, ",")
	// // stathat.Verbose = true

	// for {
	// 	time.Sleep(60 * time.Second)

	// 	if !Config.Flags.HasStatHat {
	// 		log.Println("No stathat configuration")
	// 		continue
	// 	}

	// 	log.Println("Posting to stathat")

	// 	current := qCounter.Count()
	// 	newQueries := current - lastQueryCount
	// 	lastQueryCount = current

	// 	stathat.PostEZCount("queries~"+suffix, Config.StatHat.ApiKey, int(newQueries))
	// 	stathat.PostEZValue("goroutines "+serverID, Config.StatHat.ApiKey, float64(runtime.NumGoroutine()))

	// }
}
