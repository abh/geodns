package main

import (
	metrics "github.com/abh/go-metrics"
	"log"
	"os"
	"runtime"
	"time"
)

func metricsPoster() {

	lastQueryCount := expVarToInt64(qCounter)

	queries := metrics.NewMeter()
	metrics.Register("queries", queries)

	queriesHistogram := metrics.NewHistogram(metrics.NewUniformSample(600))
	metrics.Register("queriesHistogram", queriesHistogram)

	goroutines := metrics.NewGauge()
	metrics.Register("goroutines", goroutines)

	go metrics.Log(metrics.DefaultRegistry, 30, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))

	// metrics.

	for {
		time.Sleep(1 * time.Second)

		// log.Println("updating metrics")

		current := expVarToInt64(qCounter)
		newQueries := current - lastQueryCount
		lastQueryCount = current

		queries.Mark(newQueries)
		queriesHistogram.Update(newQueries)
		goroutines.Update(int64(runtime.NumGoroutine()))

	}
}
