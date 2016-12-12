package main

import "testing"

func TestConfig(t *testing.T) {
	// check that the sample config parses
	err := configReader("dns/geodns.conf.sample")
	if err != nil {
		t.Fatalf("Could not read config: %s", err)
	}
}
