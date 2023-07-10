package main

/*
   Copyright 2012-2015 Ask Bjørn Hansen

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
	"context"
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
	"syscall"
	"time"

	"github.com/abh/geodns/v3/applog"
	"github.com/abh/geodns/v3/health"
	"github.com/abh/geodns/v3/monitor"
	"github.com/abh/geodns/v3/querylog"
	"github.com/abh/geodns/v3/server"
	"github.com/abh/geodns/v3/targeting"
	"github.com/abh/geodns/v3/targeting/geoip2"
	"github.com/abh/geodns/v3/zones"
	"github.com/pborman/uuid"
	"golang.org/x/sync/errgroup"
)

// VERSION is the current version of GeoDNS, set by the build process
var VERSION string = "devel"
var buildTime string
var gitVersion string

var (
	serverInfo *monitor.ServerInfo
)

var (
	flagconfig       = flag.String("config", "./dns/", "directory of zone files")
	flagconfigfile   = flag.String("configfile", "geodns.conf", "filename of config file (in 'config' directory)")
	flagcheckconfig  = flag.Bool("checkconfig", false, "check configuration and exit")
	flagidentifier   = flag.String("identifier", "", "identifier (hostname, pop name or similar)")
	flaginter        = flag.String("interface", "*", "set the listener address")
	flagport         = flag.String("port", "53", "default port number")
	flaghttp         = flag.String("http", ":8053", "http listen address (:8053)")
	flaglog          = flag.Bool("log", false, "be more verbose")
	flagcpus         = flag.Int("cpus", 0, "Set the maximum number of CPUs to use")
	flagLogFile      = flag.String("logfile", "", "log to file")
	flagPrivateDebug = flag.Bool("privatedebug", false, "Make debugging queries accepted only on loopback")

	flagShowVersion = flag.Bool("version", false, "Show GeoDNS version")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func init() {
	if len(gitVersion) > 0 {
		VERSION = VERSION + "/" + gitVersion
	}

	log.SetPrefix("geodns ")
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	serverInfo = &monitor.ServerInfo{}
	serverInfo.Version = VERSION
	serverInfo.UUID = uuid.New()
	serverInfo.Started = time.Now()

}

func main() {
	flag.Parse()

	if *memprofile != "" {
		runtime.MemProfileRate = 1024
	}

	if *flagShowVersion {
		extra := []string{}
		if len(buildTime) > 0 {
			extra = append(extra, buildTime)
		}
		extra = append(extra, runtime.Version())
		fmt.Printf("geodns %s (%s)\n", VERSION, strings.Join(extra, ", "))
		os.Exit(0)
	}

	if *flaglog {
		applog.Enabled = true
	}

	if len(*flagLogFile) > 0 {
		applog.FileOpen(*flagLogFile)
	}

	if len(*flagidentifier) > 0 {
		ids := strings.Split(*flagidentifier, ",")
		serverInfo.ID = ids[0]
		if len(ids) > 1 {
			serverInfo.Groups = ids[1:]
		}
	}

	var configFileName string

	if filepath.IsAbs(*flagconfigfile) {
		configFileName = *flagconfigfile
	} else {
		configFileName = filepath.Clean(filepath.Join(*flagconfig, *flagconfigfile))
	}

	if *flagcheckconfig {
		err := configReader(configFileName)
		if err != nil {
			log.Println("Errors reading config", err)
			os.Exit(2)
		}

		dirName := *flagconfig

		_, err = zones.NewMuxManager(dirName, &zones.NilReg{})
		if err != nil {
			log.Println("Errors reading zones", err)
			os.Exit(2)
		}

		// todo: setup health stuff when configured

		return
	}

	if *flagcpus > 0 {
		runtime.GOMAXPROCS(*flagcpus)
	}

	log.Printf("Starting geodns %s (%s)\n", VERSION, runtime.Version())

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		log.Printf("server shutting down")
		return nil
	})

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

	// load geodns.conf config
	err := configReader(configFileName)
	if err != nil {
		log.Printf("error reading config file %s: %s", configFileName, err)
		os.Exit(2)
	}

	if len(Config.Health.Directory) > 0 {
		go health.DirectoryReader(Config.Health.Directory)
	}

	// load (and re-load) zone data
	g.Go(func() error {
		err := configWatcher(ctx, configFileName)
		if err != nil {
			log.Printf("config watcher error: %s", err)
			return err
		}
		return nil
	})

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

	if len(Config.GeoIPDirectory()) > 0 {
		geoProvider, err := geoip2.New(Config.GeoIPDirectory())
		if err != nil {
			log.Printf("Configuring geo provider: %s", err)
		}
		if geoProvider != nil {
			targeting.Setup(geoProvider)
		}
	}

	srv := server.NewServer(serverInfo)

	if qlc := Config.AvroLog; len(qlc.Path) > 0 {

		maxsize := qlc.MaxSize
		if maxsize < 50000 {
			maxsize = 1000000
		}
		maxtime, err := time.ParseDuration(qlc.MaxTime)
		if err != nil {
			log.Printf("could not parse avrolog maxtime setting %q: %s", qlc.MaxTime, err)
		}
		if maxtime < 1*time.Second {
			maxtime = 1 * time.Second
		}

		ql, err := querylog.NewAvroLogger(qlc.Path, maxsize, maxtime)
		if err != nil {
			log.Fatalf("Could not start avro query logger: %s", err)
		}
		srv.SetQueryLogger(ql)

	} else if qlc := Config.QueryLog; len(qlc.Path) > 0 {
		ql, err := querylog.NewFileLogger(qlc.Path, qlc.MaxSize, qlc.Keep)
		if err != nil {
			log.Fatalf("Could not start file query logger: %s", err)
		}
		srv.SetQueryLogger(ql)
	}

	muxm, err := zones.NewMuxManager(*flagconfig, srv)
	if err != nil {
		log.Printf("error loading zones: %s", err)
	}

	g.Go(func() error {
		muxm.Run(ctx)
		return nil
	})

	for _, host := range inter {
		host := host
		g.Go(func() error {
			return srv.ListenAndServe(ctx, host)
		})
	}

	g.Go(func() error {
		<-ctx.Done()
		log.Printf("shutting down DNS servers")
		err = srv.Shutdown()
		if err != nil {
			return err
		}
		return nil
	})

	if len(*flaghttp) > 0 {
		g.Go(func() error {
			hs := NewHTTPServer(muxm, serverInfo)
			err := hs.Run(ctx, *flaghttp)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		log.Printf("server error: %s", err)
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
	}
	applog.FileClose()
}
