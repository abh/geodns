package zones

import (
	"math/rand"
	"testing"

	"github.com/abh/geodns/health"
	"github.com/miekg/dns"
)

type HealthStatus struct {
	t      *testing.T
	status health.StatusType
	odds   float64
}

func (hs *HealthStatus) GetStatus(name string) health.StatusType {
	hs.t.Logf("GetStatus(%s)", name)
	// hs.t.Fatalf("in get status")

	if hs.odds >= 0 {
		switch rand.Float64() < hs.odds {
		case true:
			return health.StatusHealthy
		case false:
			return health.StatusUnhealthy
		}
	}

	return hs.status
}

func (hs *HealthStatus) Close() error {
	return nil
}

func (hs *HealthStatus) Reload() error {
	return nil
}

func TestHealth(t *testing.T) {
	muxm := loadZones(t)
	t.Log("setting up health status")

	hs := &HealthStatus{t: t, odds: -1, status: health.StatusUnhealthy}

	tz := muxm.zonelist["hc.example.com"]
	tz.HealthStatus = hs
	// t.Logf("hs: '%+v'", tz.HealthStatus)
	// t.Logf("hc zone: '%+v'", tz)

	matches := tz.FindLabels("tucs", []string{"@"}, []uint16{dns.TypeA})
	// t.Logf("qt: %d, label: '%+v'", qt, label)
	records := tz.Picker(matches[0].Label, matches[0].Type, 2, nil)
	if len(records) > 0 {
		t.Errorf("got %d records when expecting 0", len(records))
	}

	// t.Logf("label.Test: '%+v'", label.Test)

	if len(records) == 0 {
		t.Log("didn't get any records")
	}
}
