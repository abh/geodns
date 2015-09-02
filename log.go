package main

import (
	"log"
	"os"
	"time"
)

type logToFile struct {
	fn      string
	file    *os.File
	closing chan chan error // channel to close the file. Pass a 'chan error' which returns the error
}

var ltf *logToFile

func newlogToFile(fn string) *logToFile {
	return &logToFile{
		fn:      fn,
		file:    nil,
		closing: make(chan chan error),
	}
}

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

func logToFileMonitor() {
	for {
		select {
		case errc := <-ltf.closing: // a close has been requested
			if ltf.file != nil {
				log.SetOutput(os.Stderr)
				ltf.file.Close()
				ltf.file = nil
			}
			errc <- nil // pass a 'nil' error back, as everything worked fine
			return
		case <-time.After(time.Duration(5 * time.Second)):
			if fi, err := os.Stat(ltf.fn); err != nil || fi.Size() == 0 {
				// it has rotated - first check we can open the new file
				if f, err := os.OpenFile(ltf.fn, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
					// Send the error to the current log file - not ideal
					log.Printf("Could not open new log file: %v", err)
				} else {
					log.SetOutput(f)
					log.Printf("Rotating log file")
					ltf.file.Close()
					ltf.file = f
				}
			}
		}
	}
}

func logToFileOpen(fn string) {

	ltf = newlogToFile(fn)

	var err error
	ltf.file, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error writing log file: %v", err)
	}
	// we deliberately do not close logFile here, because we keep it open pretty much for ever

	log.SetOutput(ltf.file)
	log.Printf("Opening log file")

	go logToFileMonitor()
}

func logToFileClose() {
	if ltf != nil {
		log.Printf("Closing log file")
		errc := make(chan error) // pass a 'chan error' through the closing channel
		ltf.closing <- errc
		_ = <-errc         // wait until the monitor has closed the log file and exited
		close(ltf.closing) // close our 'chan error' channel
		ltf = nil
	}
}
