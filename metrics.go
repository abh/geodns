package main

import (
	"log"
	"os"
	"runtime"
	"time"

	metrics "github.com/abh/go-metrics"
)

type ServerMetrics struct {
	qCounter               metrics.Meter
	lastQueryCount         int64
	queriesHistogram       metrics.Histogram
	queriesHistogramRecent metrics.Histogram
	goroutines             metrics.Gauge
}

func NewMetrics() *ServerMetrics {
	m := new(ServerMetrics)

	m.qCounter = metrics.NewMeter()
	m.lastQueryCount = m.qCounter.Count()
	metrics.Register("queries", m.qCounter)

	m.queriesHistogram = metrics.NewHistogram(metrics.NewUniformSample(1800))
	metrics.Register("queries-histogram", m.queriesHistogram)

	m.queriesHistogramRecent = metrics.NewHistogram(metrics.NewExpDecaySample(600, 0.015))
	metrics.Register("queries-histogram-recent", m.queriesHistogramRecent)

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

		// Make sure go-metrics get some input to update the rate
		m.qCounter.Mark(0)

		current := m.qCounter.Count()
		newQueries := current - m.lastQueryCount
		m.lastQueryCount = current

		m.queriesHistogram.Update(newQueries)
		m.queriesHistogramRecent.Update(newQueries)

		m.goroutines.Update(int64(runtime.NumGoroutine()))
	}
}
