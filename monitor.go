package main

import (
	"log"
	"runtime"
	"time"
)

func monitor() {
	for {
		log.Println("goroutines", runtime.NumGoroutine())
		time.Sleep(60 * time.Second)
	}
}
