package main

/*
   Copyright 2012-2014 Ask Bjørn Hansen

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// VERSION is the current version of GeoDNS
var VERSION string = "2.4.4"
var buildTime string
var gitVersion string

var serverID string
var serverIP string
var serverGroups []string

var timeStarted = time.Now()

var (
	flagconfig      = flag.String("config", "./dns/", "directory of zone files")
	flagcheckconfig = flag.Bool("checkconfig", false, "check configuration and exit")
	flagidentifier  = flag.String("identifier", "", "identifier (hostname, pop name or similar)")
	flaginter       = flag.String("interface", "*", "set the listener address")
	flagport        = flag.String("port", "53", "default port number")
	flaghttp        = flag.String("http", ":8053", "http listen address (:8053)")
	flaglog         = flag.Bool("log", false, "be more verbose")
	flagcpus        = flag.Int("cpus", 1, "Set the maximum number of CPUs to use")

	flagShowVersion = flag.Bool("version", false, "Show dnsconfig version")

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

	if *memprofile != "" {
		runtime.MemProfileRate = 1024
	}

	if *flagShowVersion {
		fmt.Println("geodns", VERSION, buildTime)
		os.Exit(0)
	}

	if len(*flagidentifier) > 0 {
		ids := strings.Split(*flagidentifier, ",")
		serverID = ids[0]
		if len(ids) > 1 {
			serverGroups = ids[1:]
		}
	}

	configFileName := filepath.Clean(*flagconfig + "/geodns.conf")

	if *flagcheckconfig {
		dirName := *flagconfig

		err := configReader(configFileName)
		if err != nil {
			log.Println("Errors reading config", err)
			os.Exit(2)
		}

		Zones := make(Zones)
		setupPgeodnsZone(Zones)
		err = zonesReadDir(dirName, Zones)
		if err != nil {
			log.Println("Errors reading zones", err)
			os.Exit(2)
		}
		return
	}

	if *flagcpus == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(*flagcpus)
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

	go configWatcher(configFileName)

	metrics := NewMetrics()
	go metrics.Updater(true)

	go statHatPoster()

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

	Zones := make(Zones)

	go monitor(Zones)
	go Zones.statHatPoster()

	setupPgeodnsZone(Zones)

	dirName := *flagconfig
	go zonesReader(dirName, Zones)

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

}
