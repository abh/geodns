package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/cyberdelia/statsd"
	"github.com/rcrowley/go-metrics"
)

func (zs *Zones) StatsDPoster() {

	if Config.StatsD.Host == "" {
		return
	}

	StatsDGroups := append(serverGroups, serverID)

	lastCounts := map[string]int64{}
	lastEdnsCounts := map[string]int64{}

	for name, zone := range *zs {
		lastCounts[name] = zone.Metrics.Queries.Count()
		lastEdnsCounts[name] = zone.Metrics.EdnsQueries.Count()
	}

	for {
		statsdSleep()

		host := fmt.Sprintf("%s:%d", Config.StatsD.Host, Config.StatsD.Port)
		c, e := statsd.Dial(host)
		if e != nil {
			logError(fmt.Sprintf("Error dialing StatsD host %s", host), e)
			continue
		}

		for name, zone := range *zs {
			if zone.Logging != nil && zone.Logging.StatsD == true {
				count := zone.Metrics.Queries.Count()
				newCount := count - lastCounts[name]
				lastCounts[name] = count

				ednsCount := zone.Metrics.EdnsQueries.Count()
				newEdnsCount := ednsCount - lastEdnsCounts[name]
				lastEdnsCounts[name] = ednsCount

				for _, serverGroup := range StatsDGroups {
					serverGroup = ipToKeyName(serverGroup)
					c.Increment(fmt.Sprintf("geodns.zone.%s.queries.%s.count", name, serverGroup), int(newCount), 1)
					c.Increment(fmt.Sprintf("geodns.zone.%s.edns_queries.%s.count", name, serverGroup), int(newEdnsCount), 1)
				}
			}
		}
		c.Flush()
		c.Close()
	}
}

func StatsDPoster() {
	qCounter := metrics.Get("queries").(metrics.Meter)
	lastQueryCount := qCounter.Count()
	StatsDGroups := append(serverGroups, serverID)
	// StatsD.Verbose = true

	for {
		statsdSleep()

		if Config.StatsD.Host == "" {
			logDebug("No StatsD configuration")
			continue
		}

		host := fmt.Sprintf("%s:%d", Config.StatsD.Host, Config.StatsD.Port)
		c, e := statsd.Dial(host) //udp
		if e != nil {
			logError(fmt.Sprintf("Error dialing StatsD host %s", host), e)
			continue
		}
		logDebug("Posting to StatsD")

		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		for _, serverGroup := range StatsDGroups {
			c.Increment(fmt.Sprintf("geodns.queries.%s.count", ipToKeyName(serverGroup)), int(newQueries), 1)
		}
		c.Gauge(fmt.Sprintf("geodns.goroutines.%s", ipToKeyName(serverID)), runtime.NumGoroutine(), 1)
		c.Flush()
		c.Close()
	}
}

func ipToKeyName(ip string) string {
	return strings.Replace(ip, ".", "-", -1)
}

func statsdSleep() {
	sleepSeconds := Config.StatsD.IntervalInSeconds
	if sleepSeconds == 0 {
		sleepSeconds = 60
	}
	time.Sleep(time.Duration(sleepSeconds) * time.Second)
}
