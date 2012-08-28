package main

import "log"

func logPrintf(format string, a ...interface{}) {
	if *flaglog {
		log.Printf(format, a...)
	}
}

func logPrintln(a ...interface{}) {
	if *flaglog {
		log.Println(a...)
	}
}
