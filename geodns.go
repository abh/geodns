package main

import (
	"expvar"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"time"
)

var VERSION string = "2.2.3"
var gitVersion string
var serverId string

var timeStarted = time.Now()
var qCounter = expvar.NewInt("qCounter")

var (
	flagconfig      = flag.String("config", "./dns/", "directory of zone files")
	flagcheckconfig = flag.Bool("checkconfig", false, "check configuration and exit")
	flaginter       = flag.String("interface", "*", "set the listener address")
	flagport        = flag.String("port", "53", "default port number")
	flaghttp        = flag.String("http", ":8053", "http listen address (:8053)")
	flaglog         = flag.Bool("log", false, "be more verbose")

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

	if *flagcheckconfig {
		dirName := *flagconfig
		Zones := make(Zones)
		setupPgeodnsZone(Zones)
		err := configReadDir(dirName, Zones)
		if err != nil {
			log.Println("Errors reading config")
			os.Exit(2)
		}
		return
	}

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

	if *flaginter == "*" {
		addrs, _ := net.InterfaceAddrs()
		ips := make([]string, 0)
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			if !(ip.IsLoopback() || ip.IsGlobalUnicast()) {
				continue
			}
			ips = append(ips, ip.String())
		}
		*flaginter = strings.Join(ips, ",")
	}

	inter := getInterfaces()

	go monitor()

	dirName := *flagconfig

	Zones := make(Zones)

	setupPgeodnsZone(Zones)

	go configReader(dirName, Zones)

	for _, host := range inter {
		go listenAndServe(host)
	}

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
