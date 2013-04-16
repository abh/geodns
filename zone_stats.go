package main

import (
	"sort"
)

type zoneLabelStats struct {
	pos     int
	rotated bool
	log     []string
	in      chan string
	out     chan []string
	reset   chan bool
	close   chan bool
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
		log:   make([]string, size),
		in:    make(chan string, 100),
		out:   make(chan []string),
		reset: make(chan bool),
		close: make(chan bool),
	}
	go zs.receiver()
	return zs
}

func (zs *zoneLabelStats) receiver() {

	for {
		select {
		case new := <-zs.in:
			zs.add(new)
		case zs.out <- zs.log:
		case <-zs.reset:
			zs.pos = 0
			zs.log = make([]string, len(zs.log))
			zs.rotated = false
		case <-zs.close:
			close(zs.in)
			return
		}
	}

}

func (zs *zoneLabelStats) Close() {
	zs.close <- true
}

func (zs *zoneLabelStats) Reset() {
	zs.reset <- true
}

func (zs *zoneLabelStats) Add(l string) {
	zs.in <- l
}

func (zs *zoneLabelStats) add(l string) {
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
	log := (<-zs.out)

	counts := make(map[string]int)
	for i, l := range log {
		if zs.rotated == false && i >= zs.pos {
			break
		}
		counts[l]++
	}
	return counts
}
