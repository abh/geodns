package main

import (
	metrics "github.com/abh/go-metrics"
	"log"
	"os"
	"runtime"
	"time"
)

var qCounter = metrics.NewMeter()

type ServerMetrics struct {
	lastQueryCount       int64
	queriesHistogram10   *metrics.StandardHistogram
	queriesHistogram60   *metrics.StandardHistogram
	queriesHistogram1440 *metrics.StandardHistogram
	goroutines           *metrics.StandardGauge
}

func NewMetrics() *ServerMetrics {
	m := new(ServerMetrics)

	m.lastQueryCount = qCounter.Count()
	metrics.Register("queries", qCounter)

	m.queriesHistogram10 = metrics.NewHistogram(metrics.NewUniformSample(600))
	metrics.Register("queries-histogram10", m.queriesHistogram10)

	m.queriesHistogram60 = metrics.NewHistogram(metrics.NewUniformSample(3600))
	metrics.Register("queries-histogram60", m.queriesHistogram60)

	m.queriesHistogram1440 = metrics.NewHistogram(metrics.NewUniformSample(24 * 60 * 60))
	metrics.Register("queries-histogram1440", m.queriesHistogram1440)

	m.goroutines = metrics.NewGauge()
	metrics.Register("goroutines", m.goroutines)

	return m
}

func (m *ServerMetrics) Updater() {

	go func() {
		time.Sleep(2 * time.Second)
		metrics.Log(metrics.DefaultRegistry, 30, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
	}()

	for {
		time.Sleep(1 * time.Second)

		// log.Println("updating metrics")

		current := qCounter.Count()
		newQueries := current - m.lastQueryCount
		m.lastQueryCount = current

		m.queriesHistogram10.Update(newQueries)
		m.queriesHistogram60.Update(newQueries)
		m.queriesHistogram1440.Update(newQueries)

		m.goroutines.Update(int64(runtime.NumGoroutine()))

	}
}
