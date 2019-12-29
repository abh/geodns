package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"github.com/nxadm/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/abh/geodns/countries"
	"github.com/abh/geodns/querylog"
)

// TODO:
// Add vendor yes/no
// add server region tag (identifier)?

const userAgent = "geodns-logs/2.0"

func main() {

	log.Printf("Starting %q", userAgent)

	identifierFlag := flag.String("identifier", "", "identifier (hostname, pop name or similar)")
	// verboseFlag := flag.Bool("verbose", false, "verbose output")
	flag.Parse()

	var serverID string
	// var serverGroups []string

	if len(*identifierFlag) > 0 {
		ids := strings.Split(*identifierFlag, ",")
		serverID = ids[0]
		if len(ids) > 1 {
			// serverGroups = ids[1:]
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

	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "geodns_logs_build_info",
			Help: "GeoDNS logs build information (in labels)",
		},
		[]string{"Version"},
	)
	prometheus.MustRegister(buildInfo)
	buildInfo.WithLabelValues(userAgent).Set(1)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":8054", nil)
		if err != nil {
			log.Printf("could not start http server: %s", err)
		}
	}()

	if len(flag.Args()) < 1 {
		log.Printf("filename to process required")
		os.Exit(2)
	}

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
	go processChan(in, nil)

	for line := range logf.Lines {
		if line.Err != nil {
			log.Printf("Error tailing file: %s", line.Err)
		}
		in <- line.Text
	}

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

func processChan(in chan string, wg *sync.WaitGroup) error {
	e := querylog.Entry{}

	stats := NewStats()

	for line := range in {
		err := json.Unmarshal([]byte(line), &e)
		if err != nil {
			log.Printf("Can't unmarshal '%s': %s", line, err)
			return err
		}
		e.Name = strings.ToLower(e.Name)

		// fmt.Printf("%s %s\n", e.Origin, e.Name)

		err = stats.Add(&e)
		if err != nil {
			return err
		}
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
	go processChan(in, &wg)

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
