package main

import (
	"flag"
	"log"
)

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")
)

func main() {

	log.SetPrefix("geodns ")
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	flag.Usage = func() {
		flag.PrintDefaults()
	}
	flag.Parse()

	dirName := "dns"

	Zones := make(Zones)

	configReader(dirName, Zones)
	runServe(&Zones)
}
