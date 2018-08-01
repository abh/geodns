package main

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestPoolCC(t *testing.T) {
	tests := []struct {
		Input    string
		Expected string
		Ok       bool
	}{
		{"pool.ntp.org", "", false},
		{"2.pool.ntp.org", "", false},
		{"us.pool.ntp.org", "us", true},
		{"0.us.pool.ntp.org", "us", true},
		{"asia.pool.ntp.org", "asia", true},
		{"3.asia.pool.ntp.org", "asia", true},
		{"3.example.pool.ntp.org", "", false},
	}

	for _, x := range tests {
		got, ok := getPoolCC(x.Input)
		if got != x.Expected {
			t.Logf("Got '%s' but expected '%s' for '%s'", got, x.Expected, x.Input)
			t.Fail()
		}
		if ok != x.Ok {
			t.Logf("Got '%t' but expected '%t' for '%s'", ok, x.Ok, x.Input)
			t.Fail()
		}
	}
}

func TestVendorName(t *testing.T) {
	tests := []struct {
		Input    string
		Expected string
	}{

		{"pool.ntp.org.", ""},
		{"2.pool.ntp.org.", ""},
		{"us.pool.ntp.org.", "_country"},
		{"2.us.pool.ntp.org.", "_country"},
		{"europe.pool.ntp.org.", "_continent"},
		{"2.europe.pool.ntp.org.", "_continent"},
		{"0.example.pool.ntp.org.", "example"},
		{"3.example.pool.ntp.org.", "example"},
	}

	for _, x := range tests {
		got := vendorName(x.Input)
		if got != x.Expected {
			t.Logf("Got '%s' but expected '%s' for '%s'", got, x.Expected, x.Input)
			t.Fail()
		}
	}
}
