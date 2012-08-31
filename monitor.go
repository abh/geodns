package main

import (
	"runtime"
	"time"
)

func monitor() {
	for {
		logPrintln("goroutines", runtime.NumGoroutine())
		time.Sleep( 60 * time.Second)
	}
}
