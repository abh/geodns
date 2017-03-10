package zones

import (
	"testing"

	"github.com/abh/geodns/health"
	"github.com/miekg/dns"
)

type HealthStatus struct {
	t *testing.T
}

func (hs *HealthStatus) GetStatus(name string) health.StatusType {
	hs.t.Logf("GetStatus(%s)", name)

	// hs.t.Fatalf("in get status")
	return health.StatusUnknown
}

func TestHealth(t *testing.T) {
	muxm := loadZones(t)
	t.Log("setting up health status")

	hs := &HealthStatus{t: t}

	tz := muxm.zonelist["hc.example.com"]
	tz.HealthStatus = hs
	// t.Logf("hs: '%+v'", tz.HealthStatus)
	// t.Logf("hc zone: '%+v'", tz)

	matches := tz.FindLabels("tucs", []string{"@"}, []uint16{dns.TypeA})
	// t.Logf("qt: %d, label: '%+v'", qt, label)
	records := tz.Picker(matches[0].Label, matches[0].Type, 2, nil)

	// t.Logf("label.Test: '%+v'", label.Test)

	t.Logf("records: '%+v'", records)

	if len(records) == 0 {
		t.Log("didn't get any records")
	}

}
