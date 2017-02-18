package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/kr/pretty"
)

const UserAgent = "geodns-logs/1.1"

type influxClient struct {
	ServerID     string
	ServerGroups []string

	URL      string
	Username string
	Password string
	Database string

	Verbose bool
	Channel chan *Stats

	wg      sync.WaitGroup
	hclient client.Client
}

func NewInfluxClient() *influxClient {
	influx := &influxClient{}
	influx.Channel = make(chan *Stats, 10)
	return influx
}

func (influx *influxClient) Start() error {
	if len(influx.URL) == 0 {
		return fmt.Errorf("InfluxDB URL required")
	}
	if len(influx.Username) == 0 {
		return fmt.Errorf("InfluxDB Username required")
	}
	if len(influx.Password) == 0 {
		return fmt.Errorf("InfluxDB Password required")
	}
	if len(influx.Database) == 0 {
		return fmt.Errorf("InfluxDB Databse required")
	}

	conf := client.HTTPConfig{
		Addr:      influx.URL,
		Username:  influx.Username,
		Password:  influx.Password,
		UserAgent: UserAgent,
	}

	hclient, err := client.NewHTTPClient(conf)
	if err != nil {
		return fmt.Errorf("Could not setup http client: %s", err)
	}
	_, _, err = hclient.Ping(time.Second * 2)
	if err != nil {
		return fmt.Errorf("Could not ping %s: %s", conf.Addr, err)

	}

	influx.hclient = hclient

	influx.wg.Add(1)
	go influx.post()

	return nil
}

func (influx *influxClient) Close() {
	close(influx.Channel)
	influx.wg.Wait()
}

func (influx *influxClient) post() {
	hclient := influx.hclient

	for stats := range influx.Channel {
		if influx.Verbose {
			pretty.Println("Sending", stats)
		}
		log.Printf("Sending %d stats points", len(stats.Map))

		batch, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database:        "geodns_logs",
			RetentionPolicy: "incoming",
		})
		if err != nil {
			log.Printf("Could not setup batch points: %s", err)
			continue
		}

		for _, s := range stats.Map {
			pnt, err := client.NewPoint(
				"log_stats",
				map[string]string{
					"Label":  s.Label,
					"Name":   s.Name,
					"Origin": s.Origin,
					"PoolCC": s.PoolCC,
					"Vendor": s.Vendor,
					"Qtype":  s.Qtype,
					"Server": influx.ServerID,
				},
				map[string]interface{}{
					"Count": s.Count,
				},
				time.Unix(s.Time, 0),
			)
			if err != nil {
				log.Printf("Could not create a point from '%+v': %s", s, err)
				continue
			}
			batch.AddPoint(pnt)
		}

		err = hclient.Write(batch)
		if err != nil {
			log.Printf("Error writing batch points: %s", err)
		}
	}

	influx.wg.Done()
}
