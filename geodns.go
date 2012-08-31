package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
)

var VERSION string = "2.0"
var gitVersion string

var timeStarted = time.Now()
var qCounter uint64 = 0

var (
	listen  = flag.String("listen", ":8053", "set the listener address")
	flaglog = flag.Bool("log", false, "be more verbose")
	flagrun = flag.Bool("run", false, "run server")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func init() {
	if len(gitVersion) > 0 {
		VERSION = VERSION + "/" + gitVersion
	}

	log.SetPrefix("geodns ")
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
}

func main() {
	flag.Parse()

	log.Printf("Starting geodns %s\n", VERSION)

	if *cpuprofile != "" {
		prof, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err.Error())
		}

		pprof.StartCPUProfile(prof)
		defer func() {
			log.Println("closing file")
			prof.Close()
		}()
		defer func() {
			log.Println("stopping profile")
			pprof.StopCPUProfile()
		}()
	}

	go monitor()

	dirName := "dns"

	Zones := make(Zones)

	setupPgeodnsZone(Zones)

	go configReader(dirName, Zones)
	go listenAndServe(&Zones)

	if *flagrun {
		terminate := make(chan os.Signal)
		signal.Notify(terminate, os.Interrupt)

		<-terminate
		log.Printf("geodns: signal received, stopping")

		if *memprofile != "" {
			f, err := os.Create(*memprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}

		//os.Exit(0)
	}
}
