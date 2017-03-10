package health

import (
	"fmt"

	"github.com/abh/geodns/typeutil"
)

type HealthTester interface {
	// Test(record string) bool
	Name(record string) string
	String() string
}

type HealthReference struct {
	name string
}

func (hr *HealthReference) Name(record string) string {
	if len(record) > 0 {
		return hr.name + "/" + record
	}
	return hr.name
}

func (hr *HealthReference) String() string {
	return hr.name
}

func NewReferenceFromMap(i map[string]interface{}) (HealthTester, error) {
	var name, ts string

	if ti, ok := i["type"]; ok {
		ts = typeutil.ToString(ti)
	}

	if ni, ok := i["name"]; ok {
		name = typeutil.ToString(ni)
	}

	if len(name) == 0 {
		name = ts
	}

	if len(name) == 0 {
		return nil, fmt.Errorf("name or type required")
	}

	tester := &HealthReference{name: name}
	return tester, nil
}

// func (hr *HealthReference) RecordTest(rec *zones.Record) {
// 	key := ht.String()
// 	htr.entryMutex.Lock()
// 	defer htr.entryMutex.Unlock()
// 	if t, ok := htr.entries[key]; ok {
// 		// we already have an instance of this test running. Record we are using it
// 		t.references[ref] = true
// 	} else {
// 		// a test that isn't running. Record we are using it and start the test
// 		t := &HealthTestRunnerEntry{
// 			HealthTest: *ht.copy(ht.ipAddress),
// 			references: make(map[string]bool),
// 		}
// 		if t.global {
// 			t.ipAddress = nil
// 		}
// 		// we know it is not started, so no need for the mutex
// 		t.healthy = ht.healthy
// 		t.references[ref] = true
// 		t.start()
// 		htr.entries[key] = t
// 	}
// }
