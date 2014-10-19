package main

import (
	"runtime"
	"time"

	metrics "github.com/rcrowley/go-metrics"
)

type ServerMetrics struct {
	qCounter         metrics.Meter
	lastQueryCount   int64
	queriesHistogram metrics.Histogram
	goroutines       metrics.Gauge
}

func NewMetrics() *ServerMetrics {
	m := new(ServerMetrics)

	m.qCounter = metrics.GetOrRegisterMeter("queries", nil)
	m.lastQueryCount = m.qCounter.Count()

	m.queriesHistogram = metrics.GetOrRegisterHistogram(
		"queries-histogram", nil,
		metrics.NewExpDecaySample(600, 0.015),
	)

	m.goroutines = metrics.GetOrRegisterGauge("goroutines", nil)

	return m
}

func (m *ServerMetrics) Updater() {

	for {
		time.Sleep(1 * time.Second)

		// Make sure go-metrics get some input to update the rate
		m.qCounter.Mark(0)

		current := m.qCounter.Count()
		newQueries := current - m.lastQueryCount
		m.lastQueryCount = current

		m.queriesHistogram.Update(newQueries)

		m.goroutines.Update(int64(runtime.NumGoroutine()))
	}
}
