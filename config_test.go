package main

import (
	"testing"

	"github.com/abh/geodns/v3/appconfig"
)

func TestConfig(t *testing.T) {
	// check that the sample config parses
	err := appconfig.ConfigReader("dns/geodns.conf.sample")
	if err != nil {
		t.Fatalf("Could not read config: %s", err)
	}
}
