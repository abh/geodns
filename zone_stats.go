package main

import (
	"sort"
	"sync"
)

type zoneLabelStats struct {
	pos     int
	rotated bool
	log     []string
	mu      sync.Mutex
}

type labelStats []labelStat

func (s labelStats) Len() int      { return len(s) }
func (s labelStats) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type labelStatsByCount struct{ labelStats }

func (s labelStatsByCount) Less(i, j int) bool { return s.labelStats[i].Count > s.labelStats[j].Count }

type labelStat struct {
	Label string
	Count int
}

func NewZoneLabelStats(size int) *zoneLabelStats {
	zs := &zoneLabelStats{
		log: make([]string, size),
	}
	return zs
}

func (zs *zoneLabelStats) Close() {
	zs.log = []string{}
}

func (zs *zoneLabelStats) Reset() {
	zs.mu.Lock()
	defer zs.mu.Unlock()
	zs.pos = 0
	zs.log = make([]string, len(zs.log))
	zs.rotated = false
}

func (zs *zoneLabelStats) Add(l string) {
	zs.add(l)
}

func (zs *zoneLabelStats) add(l string) {
	zs.mu.Lock()
	defer zs.mu.Unlock()

	zs.log[zs.pos] = l
	zs.pos++
	if zs.pos+1 > len(zs.log) {
		zs.rotated = true
		zs.pos = 0
	}
}

func (zs *zoneLabelStats) TopCounts(n int) labelStats {
	cm := zs.Counts()
	top := make(labelStats, len(cm))
	i := 0
	for l, c := range cm {
		top[i] = labelStat{l, c}
		i++
	}
	sort.Sort(labelStatsByCount{top})
	if len(top) > n {
		others := 0
		for _, t := range top[n:] {
			others += t.Count
		}
		top = append(top[:n], labelStat{"Others", others})
	}
	return top
}

func (zs *zoneLabelStats) Counts() map[string]int {
	zs.mu.Lock()
	defer zs.mu.Unlock()

	counts := make(map[string]int)
	for i, l := range zs.log {
		if zs.rotated == false && i >= zs.pos {
			break
		}
		counts[l]++
	}
	return counts
}
