package main

import "log"

func logDebugf(format string, a ...interface{}) {
	if *flaglog {
		log.Printf(format, a...)
	}
}

func logDebug(a ...interface{}) {
	if *flaglog {
		log.Println(a...)
	}
}

// always print
func logError(msg string, err error) {
  log.Printf("Error: %s (%v)\n", msg, err)
}
