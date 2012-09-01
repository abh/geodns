package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"time"
)

var VERSION string = "2.0"
var gitVersion string
var serverId string

var timeStarted = time.Now()
var qCounter uint64 = 0

var (
	flagint  = flag.String("interface", "*", "set the listener address")
	flagport = flag.String("port", "53", "default port number")
	flaglog  = flag.Bool("log", false, "be more verbose")
	flagrun  = flag.Bool("run", false, "run server")

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
	for _, host := range strings.Split(*flagint, ",") {
		ip, port, err := net.SplitHostPort(host)
		if err != nil {
			switch {
			case strings.Contains(err.Error(), "missing port in address"):
				// 127.0.0.1
				ip = host
			case strings.Contains(err.Error(), "too many colons in address") &&
				// [a:b::c]
				strings.LastIndex(host, "]") == len(host)-1:
				ip = host[1 : len(host)-1]
				port = ""
			default:
				log.Fatalf("Could not parse %s: %s\n", host, err)
			}
		}
		if len(port) == 0 {
			port = *flagport
		}
		host = net.JoinHostPort(ip, port)
		if len(serverId) == 0 {
			serverId = ip
		}
		go listenAndServe(host, &Zones)
	}

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
