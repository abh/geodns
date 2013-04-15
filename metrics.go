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
	lastQueryCount         int64
	queriesHistogram       *metrics.StandardHistogram
	queriesHistogramRecent *metrics.StandardHistogram
	goroutines             *metrics.StandardGauge
}

func NewMetrics() *ServerMetrics {
	m := new(ServerMetrics)

	m.lastQueryCount = qCounter.Count()
	metrics.Register("queries", qCounter)

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
		qCounter.Mark(0)

		current := qCounter.Count()
		newQueries := current - m.lastQueryCount
		m.lastQueryCount = current

		m.queriesHistogram.Update(newQueries)
		m.queriesHistogramRecent.Update(newQueries)

		m.goroutines.Update(int64(runtime.NumGoroutine()))

	}
}
