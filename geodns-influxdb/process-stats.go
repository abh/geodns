package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/abh/geodns/countries"
	"github.com/abh/geodns/querylog"
)

// TODO:
// Add vendor yes/no
// add server region tag (identifier)?

func main() {

	tailFlag := flag.Bool("tail", false, "tail the log file instead of processing all arguments")
	identifierFlag := flag.String("identifier", "", "identifier (hostname, pop name or similar)")
	verboseFlag := flag.Bool("verbose", false, "verbose output")
	flag.Parse()

	var serverID string
	var serverGroups []string

	if len(*identifierFlag) > 0 {
		ids := strings.Split(*identifierFlag, ",")
		serverID = ids[0]
		if len(ids) > 1 {
			serverGroups = ids[1:]
		}
	}

	if len(serverID) == 0 {
		var err error
		serverID, err = os.Hostname()
		if err != nil {
			log.Printf("Could not get hostname: %s", err)
			os.Exit(2)
		}
	}

	queries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_logs_total",
			Help: "Number of served queries",
		},
		[]string{"zone", "vendor", "usercc", "poolcc", "qtype"},
	)
	prometheus.MustRegister(queries)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8054", nil)

	influx := NewInfluxClient()
	influx.URL = os.Getenv("INFLUXDB_URL")
	influx.Username = os.Getenv("INFLUXDB_USERNAME")
	influx.Password = os.Getenv("INFLUXDB_PASSWORD")
	influx.Database = os.Getenv("INFLUXDB_DATABASE")

	influx.ServerID = serverID
	influx.ServerGroups = serverGroups
	influx.Verbose = *verboseFlag

	err := influx.Start()
	if err != nil {
		log.Printf("Could not start influxdb poster: %s", err)
		os.Exit(2)
	}

	if len(flag.Args()) < 1 {
		log.Printf("filename to process required")
		os.Exit(2)
	}

	if *tailFlag {

		filename := flag.Arg(0)

		logf, err := tail.TailFile(filename, tail.Config{
			// Location:  &tail.SeekInfo{-1, 0},
			Poll:      true, // inotify is flaky on EL6, so try this ...
			ReOpen:    true,
			MustExist: false,
			Follow:    true,
		})
		if err != nil {
			log.Printf("Could not tail '%s': %s", filename, err)
		}

		in := make(chan string)

		go processChan(in, influx.Channel, nil)

		for line := range logf.Lines {
			if line.Err != nil {
				log.Printf("Error tailing file: %s", line.Err)
			}
			in <- line.Text
		}
	} else {
		for _, file := range flag.Args() {
			log.Printf("Log: %s", file)
			err := processFile(file, influx.Channel)
			if err != nil {
				log.Printf("Error processing '%s': %s", file, err)
			}
			log.Printf("Done with %s", file)
		}
	}

	influx.Close()
}

var extraValidLabels = map[string]struct{}{
	"uk":       struct{}{},
	"_status":  struct{}{},
	"_country": struct{}{},
	"www":      struct{}{},
	"nag-test": struct{}{},
}

func validCC(label string) bool {
	if _, ok := countries.CountryContinent[label]; ok {
		return true
	}
	if _, ok := countries.ContinentCountries[label]; ok {
		return true
	}
	if _, ok := countries.RegionGroupRegions[label]; ok {
		return true
	}
	if _, ok := countries.RegionGroups[label]; ok {
		return true
	}
	if _, ok := extraValidLabels[label]; ok {
		return true
	}
	return false
}

func getPoolCC(label string) (string, bool) {
	l := dns.SplitDomainName(label)
	// log.Printf("LABEL: %+v", l)
	if len(l) == 0 {
		return "", true
	}

	for _, cc := range l {
		if validCC(cc) {
			return cc, true
		}
	}

	if len(l[0]) == 1 && strings.ContainsAny(l[0], "01234") {
		if len(l) == 1 {
			return "", true
		}
	}

	// log.Printf("LABEL '%s' unhandled cc...", label)
	return "", false
}

func processChan(in chan string, out chan<- *Stats, wg *sync.WaitGroup) error {
	e := querylog.Entry{}

	// the grafana queries depend on this being one minute
	submitInterval := time.Minute * 1

	stats := NewStats()
	i := 0
	lastMinute := int64(0)
	for line := range in {
		err := json.Unmarshal([]byte(line), &e)
		if err != nil {
			log.Printf("Can't unmarshal '%s': %s", line, err)
			return err
		}
		e.Name = strings.ToLower(e.Name)

		eMinute := ((e.Time - e.Time%int64(submitInterval)) / int64(time.Second))
		e.Time = eMinute

		if len(stats.Map) == 0 {
			lastMinute = eMinute
			log.Printf("Last Minute: %d", lastMinute)
		} else {
			if eMinute > lastMinute {
				fmt.Printf("eMinute %d\nlastMin %d - should summarize\n", eMinute, lastMinute)

				stats.Summarize()
				out <- stats
				stats = NewStats()
				lastMinute = eMinute
			}
		}

		// fmt.Printf("%s %s\n", e.Origin, e.Name)

		err = stats.Add(&e)
		if err != nil {
			return err
		}

		if i%10000 == 0 {
			// pretty.Println(stats)
		}
		// minute
	}

	if len(stats.Map) > 0 {
		out <- stats
	}
	if wg != nil {
		wg.Done()
	}
	return nil
}

func processFile(file string, out chan<- *Stats) error {
	fh, err := os.Open(file)
	if err != nil {
		return err
	}

	in := make(chan string)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go processChan(in, out, &wg)

	scanner := bufio.NewScanner(fh)

	for scanner.Scan() {
		in <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		log.Println("reading standard input:", err)
	}

	close(in)

	wg.Wait()

	return nil
}
