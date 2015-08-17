package main

import (
	"fmt"
	"github.com/abh/geodns/Godeps/_workspace/src/github.com/miekg/dns"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	healthQtypes = []uint16{dns.TypeA, dns.TypeAAAA}
)

type HealthTester interface {
	Test(ht *HealthTest) bool
	String() string
}

type HealthTestParameters struct {
	frequency        time.Duration
	retryTime        time.Duration
	timeout          time.Duration
	retries          int
	healthyInitially bool
	testName         string
	global           bool
}

type HealthTest struct {
	HealthTestParameters
	ipAddress    net.IP
	healthy      bool
	healthyMutex sync.RWMutex
	closing      chan chan error
	health       chan bool
	tester       *HealthTester
	globalMap    map[string]bool
}

type HealthTestRunnerEntry struct {
	HealthTest
	references map[string]bool
}

type HealthTestRunner struct {
	entries    map[string]*HealthTestRunnerEntry
	entryMutex sync.RWMutex
}

var healthTestRunner = &HealthTestRunner{
	entries: make(map[string]*HealthTestRunnerEntry),
}

func defaultHealthTestParameters() HealthTestParameters {
	return HealthTestParameters{
		frequency:        30 * time.Second,
		retryTime:        5 * time.Second,
		timeout:          5 * time.Second,
		retries:          3,
		healthyInitially: false,
	}
}

func newHealthTest(ipAddress net.IP, htp HealthTestParameters, tester *HealthTester) *HealthTest {
	ht := HealthTest{
		ipAddress:            ipAddress,
		HealthTestParameters: htp,
		healthy:              true,
		tester:               tester,
		globalMap:            make(map[string]bool),
	}
	ht.healthy = ht.healthyInitially
	if ht.frequency < time.Second {
		ht.frequency = time.Second
	}
	if ht.retryTime < time.Second {
		ht.retryTime = time.Second
	}
	if ht.timeout < time.Second {
		ht.timeout = time.Second
	}
	return &ht
}

// Format the health test as a string - used to compare two tests and as an index for the hash
func (ht *HealthTest) String() string {
	ip := ht.ipAddress.String()
	if ht.HealthTestParameters.global {
		ip = "" // ensure we have a single instance of a global health check with the same paramaters
	}
	return fmt.Sprintf("%s/%v/%s", ip, ht.HealthTestParameters, (*ht.tester).String())
}

// safe copy function that copies the parameters but not (e.g.) the
// mutex
func (ht *HealthTest) copy(ipAddress net.IP) *HealthTest {
	return newHealthTest(ipAddress, ht.HealthTestParameters, ht.tester)
}

func (ht *HealthTest) setGlobal(g map[string]bool) {
	ht.healthyMutex.Lock()
	defer ht.healthyMutex.Unlock()
	ht.globalMap = g
}

func (ht *HealthTest) getGlobal(k string) (bool, bool) {
	ht.healthyMutex.RLock()
	defer ht.healthyMutex.RUnlock()
	healthy, ok := ht.globalMap[k]
	return healthy, ok
}

func (ht *HealthTest) run() {
	randomDelay := rand.Int63n(ht.frequency.Nanoseconds())
	if !ht.isHealthy() {
		randomDelay = rand.Int63n(ht.retryTime.Nanoseconds())
	}
	var nextPoll time.Time = time.Now().Add(time.Duration(randomDelay))
	var pollStart time.Time
	failCount := 0
	for {
		var pollDelay time.Duration
		if now := time.Now(); nextPoll.After(now) {
			pollDelay = nextPoll.Sub(now)
		}
		var startPoll <-chan time.Time
		var closingPoll <-chan chan error
		if pollStart.IsZero() {
			closingPoll = ht.closing
			startPoll = time.After(pollDelay)
		}
		select {
		case errc := <-closingPoll: // don't close while we are polling or we send to a closed channel
			errc <- nil
			return
		case <-startPoll:
			pollStart = time.Now()
			go ht.poll()
		case h := <-ht.health:
			nextPoll = pollStart.Add(ht.frequency)
			if h {
				ht.setHealthy(true)
				failCount = 0
			} else {
				failCount++
				logPrintf("Failure for %s, retry count=%d, healthy=%v", ht.ipAddress, failCount, ht.isHealthy())
				if failCount >= ht.retries {
					ht.setHealthy(false)
					nextPoll = pollStart.Add(ht.retryTime)
				}
			}
			pollStart = time.Time{}
			logPrintf("Check result for %s health=%v, next poll at %s", ht.ipAddress, h, nextPoll)
			//randomDelay := rand.Int63n(time.Second.Nanoseconds())
			//nextPoll = nextPoll.Add(time.Duration(randomDelay))
		}
	}
}

func (ht *HealthTest) poll() {
	logPrintf("Checking health of %s", ht.ipAddress)
	result := (*ht.tester).Test(ht)
	logPrintf("Checked health of %s, healthy=%v", ht.ipAddress, result)
	ht.health <- result
}

func (ht *HealthTest) start() {
	ht.closing = make(chan chan error)
	ht.health = make(chan bool)
	logPrintf("Starting health test on %s, frequency=%s, retry_time=%s, timeout=%s, retries=%d", ht.ipAddress, ht.frequency, ht.retryTime, ht.timeout, ht.retries)
	go ht.run()
}

func (ht *HealthTest) stop() (err error) {
	// Check it's been started by existing of the closing channel
	if ht.closing == nil {
		return nil
	}
	logPrintf("Stopping health test on %s", ht.ipAddress)
	errc := make(chan error)
	ht.closing <- errc
	err = <-errc
	close(ht.closing)
	ht.closing = nil
	close(ht.health)
	ht.health = nil
	return err
}

func (ht *HealthTest) isHealthy() bool {
	ht.healthyMutex.RLock()
	h := ht.healthy
	ht.healthyMutex.RUnlock()
	return h
}

func (ht *HealthTest) setHealthy(h bool) {
	ht.healthyMutex.Lock()
	old := ht.healthy
	ht.healthy = h
	ht.healthyMutex.Unlock()
	if old != h {
		logPrintf("Changing health status of %s from %v to %v", ht.ipAddress, old, h)
	}
}

func (htr *HealthTestRunner) addTest(ht *HealthTest, ref string) {
	key := ht.String()
	htr.entryMutex.Lock()
	defer htr.entryMutex.Unlock()
	if t, ok := htr.entries[key]; ok {
		// we already have an instance of this test running. Record we are using it
		t.references[ref] = true
	} else {
		// a test that isn't running. Record we are using it and start the test
		t := &HealthTestRunnerEntry{
			HealthTest: *ht.copy(ht.ipAddress),
			references: make(map[string]bool),
		}
		if t.global {
			t.ipAddress = nil
		}
		// we know it is not started, so no need for the mutex
		t.healthy = ht.healthy
		t.references[ref] = true
		t.start()
		htr.entries[key] = t
	}
}

func (htr *HealthTestRunner) removeTest(ht *HealthTest, ref string) {
	key := ht.String()
	htr.entryMutex.Lock()
	defer htr.entryMutex.Unlock()
	if t, ok := htr.entries[key]; ok {
		delete(t.references, ref)
		// record the last state of health
		ht.healthyMutex.Lock()
		ht.healthy = t.isHealthy()
		ht.healthyMutex.Unlock()
		if len(t.references) == 0 {
			// no more references, delete the test
			t.stop()
			delete(htr.entries, key)
		}
	}
}

func (htr *HealthTestRunner) refAllGlobalHealthChecks(ref string, add bool) {
	htr.entryMutex.Lock()
	defer htr.entryMutex.Unlock()
	for key, t := range htr.entries {
		if t.global {
			if add {
				t.references[ref] = true
			} else {
				delete(t.references, ref)
				if len(t.references) == 0 {
					// no more references, delete the test
					t.stop()
					delete(htr.entries, key)
				}
			}
		}
	}
}

func (htr *HealthTestRunner) isHealthy(ht *HealthTest) bool {
	key := ht.String()
	htr.entryMutex.RLock()
	defer htr.entryMutex.RUnlock()
	if t, ok := htr.entries[key]; ok {
		if t.global {
			healthy, ok := t.getGlobal(ht.ipAddress.String())
			if ok {
				return healthy
			}
		} else {
			return t.isHealthy()
		}
	}
	return ht.isHealthy()
}

func (z *Zone) newHealthTest(l *Label, data interface{}) {
	// First safely get rid of any old test. As label tests
	// should never run this should never be executed
	if l.Test != nil {
		l.Test.stop()
		l.Test = nil
	}

	if data == nil {
		return
	}
	if i, ok := data.(map[string]interface{}); ok {
		if t, ok := i["type"]; ok {
			ts := valueToString(t)
			htp := defaultHealthTestParameters()
			if nh, ok := HealthTesterMap[ts]; !ok {
				log.Printf("Bad health test type '%s'", ts)
			} else {
				htp.testName = ts
				h := nh(i, &htp)

				for k, v := range i {
					switch k {
					case "frequency":
						htp.frequency = time.Duration(valueToInt(v)) * time.Second
					case "retry_time":
						htp.retryTime = time.Duration(valueToInt(v)) * time.Second
					case "timeout":
						htp.retryTime = time.Duration(valueToInt(v)) * time.Second
					case "retries":
						htp.retries = valueToInt(v)
					case "healthy_initially":
						htp.healthyInitially = valueToBool(v)
						logPrintf("HealthyInitially for %s is %v", l.Label, htp.healthyInitially)
					}
				}

				l.Test = newHealthTest(nil, htp, &h)
			}
		}
	}
}

func (z *Zone) StartStopHealthChecks(start bool, oldZone *Zone) {
	logPrintf("Start/stop health checks on zone %s start=%v", z.Origin, start)
	for labelName, label := range z.Labels {
		for _, qtype := range healthQtypes {
			if label.Records[qtype] != nil && len(label.Records[qtype]) > 0 {
				for i := range label.Records[qtype] {
					rr := label.Records[qtype][i].RR
					var ip net.IP
					switch rrt := rr.(type) {
					case *dns.A:
						ip = rrt.A
					case *dns.AAAA:
						ip = rrt.AAAA
					default:
						continue
					}
					var test *HealthTest
					ref := fmt.Sprintf("%s/%s/%d/%d", z.Origin, labelName, qtype, i)
					if start {
						if test = label.Records[qtype][i].Test; test != nil {
							// stop any old test
							healthTestRunner.removeTest(test, ref)
						} else {
							if ltest := label.Test; ltest != nil {
								test = ltest.copy(ip)
								label.Records[qtype][i].Test = test
							}
						}
						if test != nil {
							test.ipAddress = ip
							// if we are given an oldzone, let's see if we can find the old RR and
							// copy over the initial health state, rather than use the initial health
							// state provided from the label. This helps to stop health state bouncing
							// when a zone file is reloaded for a purposes unrelated to the RR
							if oldZone != nil {
								oLabel, ok := oldZone.Labels[labelName]
								if ok {
									if oLabel.Test != nil {
										for i := range oLabel.Records[qtype] {
											oRecord := oLabel.Records[qtype][i]
											var oip net.IP
											switch orrt := oRecord.RR.(type) {
											case *dns.A:
												oip = orrt.A
											case *dns.AAAA:
												oip = orrt.AAAA
											default:
												continue
											}
											if oip.Equal(ip) {
												if oRecord.Test != nil {
													h := oRecord.Test.isHealthy()
													logPrintf("Carrying over previous health state for %s: %v", oRecord.Test.ipAddress, h)
													// we know the test is stopped (as we haven't started it) so we can write
													// without the mutex and avoid a misleading log message
													test.healthy = h
												}
												break
											}
										}
									}
								}
							}
							healthTestRunner.addTest(test, ref)
						}
					} else {
						if test = label.Records[qtype][i].Test; test != nil {
							healthTestRunner.removeTest(test, ref)
						}
					}
				}
			}
		}
	}
}
