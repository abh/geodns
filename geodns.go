package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
)

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")
)

func main() {

	log.SetPrefix("geodns ")
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	dirName := "dns"

	Zones := make(Zones)

	go configReader(dirName, Zones)
	go startServer(&Zones)

	if *flagrun {
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt)

		<-sig
		log.Printf("geodns: signal received, stopping")
		os.Exit(0)
	}

}
