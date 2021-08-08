package main

import "testing"

func TestGetPoolCC(t *testing.T) {
	tests := []struct {
		label string
		cc    string
		ok    bool
	}{
		{"1.debian.pool.ntp.org.", "", false},
		{"2.dk.pool.ntp.org.", "dk", true},
		{"dk.pool.ntp.org.", "dk", true},
		{"0.asia.pool.ntp.org.", "asia", true},
		{"1.pool.ntp.org.", "", false},
	}

	for _, input := range tests {
		cc, ok := getPoolCC(input.label)
		if cc != input.cc || ok != input.ok {
			t.Logf("%q got %q (%t), expected %q (%t)", input.label, cc, ok, input.cc, input.ok)
			t.Fail()
		}
	}
}
