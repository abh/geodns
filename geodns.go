package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"time"
)

const VERSION = "2.0"

var timeStarted = time.Now()
var qCounter uint64 = 0

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")
)

func main() {

	log.SetPrefix("geodns ")
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	log.Printf("Starting geodns/%s\n", VERSION)

	dirName := "dns"

	Zones := make(Zones)

	go configReader(dirName, Zones)
	go listenAndServe(&Zones)

	if *flagrun {
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt)

		<-sig
		log.Printf("geodns: signal received, stopping")
		os.Exit(0)
	}

}
