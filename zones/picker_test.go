package zones

import (
	"testing"

	dns "codeberg.org/miekg/dns"
)

func pickZone(weights []int) (*Zone, *Label, Records) {
	zone := NewZone("example.com")
	label := zone.AddLabel("pick")
	recs := make(Records, len(weights))
	sum := 0
	for i, w := range weights {
		recs[i] = &Record{Weight: w}
		sum += w
	}
	label.Records[dns.TypeA] = recs
	label.Weight[dns.TypeA] = sum
	return zone, label, recs
}

func weightsOf(recs Records) []int {
	out := make([]int, len(recs))
	for i, r := range recs {
		if r == nil {
			out[i] = -999
			continue
		}
		out[i] = r.Weight
	}
	return out
}

func sameRecords(got Records, want ...*Record) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// A zero total returns before the draw and ignores max.
func TestPickerSumZeroReturnsEveryRecord(t *testing.T) {
	zone, label, recs := pickZone([]int{0, 0, 0})

	got := zone.Picker(label, dns.TypeA, 1, nil)
	if !sameRecords(got, recs...) {
		t.Fatalf("got %#v", got)
	}
}

// Positive weight is removed until sum hits 0, then zero-weight records fill in order.
func TestPickerSumZeroFillsInOrder(t *testing.T) {
	zone, label, recs := pickZone([]int{10, 0, 0})

	got := zone.Picker(label, dns.TypeA, 2, nil)
	if !sameRecords(got, recs[0], recs[1]) {
		t.Fatalf("weights %v", weightsOf(got))
	}

	got = zone.Picker(label, dns.TypeA, 3, nil)
	if !sameRecords(got, recs...) {
		t.Fatalf("weights %v", weightsOf(got))
	}
}

// label.Weight < 0 is not turned into the sum == 0 early return.
func TestPickerNegativeSumLeavesNilSlots(t *testing.T) {
	zone, label, recs := pickZone([]int{-2, -4})
	got := zone.Picker(label, dns.TypeA, 2, nil)
	if len(got) != 2 || got[0] != nil || got[1] != nil {
		t.Fatalf("all-negative got %#v", got)
	}

	zone, label, recs = pickZone([]int{10, -3})
	got = zone.Picker(label, dns.TypeA, 1, nil)
	if !sameRecords(got, recs[0]) {
		t.Fatalf("single slot got %#v", got)
	}

	got = zone.Picker(label, dns.TypeA, 2, nil)
	if len(got) != 2 || got[0] != recs[0] || got[1] != nil {
		t.Fatalf("mixed got %v nil=%v", weightsOf(got), got[1] == nil)
	}

	// A later positive record is still drawn; the leftover negative slot is nil.
	zone, label, recs = pickZone([]int{-1, 4})
	got = zone.Picker(label, dns.TypeA, 2, nil)
	if len(got) != 2 || got[0] != recs[1] || got[1] != nil {
		t.Fatalf("negative-then-positive got %#v", got)
	}
}

// Weights that cancel are a zero total, so both records return and max is ignored.
func TestPickerCancellingWeightsReturnAll(t *testing.T) {
	zone, label, recs := pickZone([]int{5, -5})
	got := zone.Picker(label, dns.TypeA, 1, nil)
	if !sameRecords(got, recs...) {
		t.Fatalf("got %#v", got)
	}
}

func TestPickerMaxHostsAboveRecordCount(t *testing.T) {
	zone, label, recs := pickZone([]int{4, 6})
	got := zone.Picker(label, dns.TypeA, 9, nil)
	if len(got) != 2 || got[0] == nil || got[1] == nil {
		t.Fatalf("got %#v", got)
	}
	seen := map[*Record]int{}
	for _, r := range got {
		seen[r]++
	}
	if seen[recs[0]] != 1 || seen[recs[1]] != 1 {
		t.Fatalf("got %v", weightsOf(got))
	}

	zone, label, recs = pickZone([]int{4})
	got = zone.Picker(label, dns.TypeA, 9, nil)
	if !sameRecords(got, recs[0]) {
		t.Fatalf("got %#v", got)
	}

	zone, label, _ = pickZone([]int{10, 0, 0})
	got = zone.Picker(label, dns.TypeA, 0, nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("max 0 got %#v", got)
	}
}

func TestPickerWeightedSelection(t *testing.T) {
	zone, label, recs := pickZone([]int{7})
	for i := 0; i < 20; i++ {
		got := zone.Picker(label, dns.TypeA, 1, nil)
		if !sameRecords(got, recs[0]) {
			t.Fatalf("single record got %#v", got)
		}
	}

	// While positive weight remains, a zero-weight record is not drawn.
	zone, label, recs = pickZone([]int{10, 0})
	for i := 0; i < 20; i++ {
		got := zone.Picker(label, dns.TypeA, 1, nil)
		if !sameRecords(got, recs[0]) {
			t.Fatalf("pick %d got %#v", i, got)
		}
	}

	zone, label, recs = pickZone([]int{10, 20})
	for i := 0; i < 30; i++ {
		got := zone.Picker(label, dns.TypeA, 2, nil)
		if len(got) != 2 || got[0] == nil || got[1] == nil || got[0] == got[1] {
			t.Fatalf("pair got %#v", got)
		}
		if (got[0] != recs[0] && got[0] != recs[1]) || (got[1] != recs[0] && got[1] != recs[1]) {
			t.Fatalf("pair got %#v", got)
		}
	}

	// Equal weights must not keep the old extra slot on the first record (2/3).
	zone, label, recs = pickZone([]int{1, 1})
	const trials = 4000
	first := 0
	for i := 0; i < trials; i++ {
		got := zone.Picker(label, dns.TypeA, 1, nil)
		if len(got) != 1 || got[0] == nil {
			t.Fatalf("got %#v", got)
		}
		if got[0] == recs[0] {
			first++
		}
	}
	if first < trials*45/100 || first > trials*55/100 {
		t.Fatalf("first record selected %d/%d times", first, trials)
	}

	// 1:3 stays near 1/4. The old draw would give a leading weight-1 record 2/5.
	zone, label, recs = pickZone([]int{1, 3})
	light := 0
	for i := 0; i < trials; i++ {
		got := zone.Picker(label, dns.TypeA, 1, nil)
		if len(got) != 1 || (got[0] != recs[0] && got[0] != recs[1]) {
			t.Fatalf("got %#v", got)
		}
		if got[0] == recs[0] {
			light++
		}
	}
	if light < trials*18/100 || light > trials*32/100 {
		t.Fatalf("weight-1 record selected %d/%d times", light, trials)
	}
}
