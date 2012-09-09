package main

import (
	"log"
	"runtime"
	"time"
)

func monitor() {
	lastQueryCount := qCounter
	for {
		newQueries := qCounter - lastQueryCount
		lastQueryCount = qCounter
		log.Println("goroutines", runtime.NumGoroutine(), "queries", newQueries)
		time.Sleep(60 * time.Second)
	}
}
