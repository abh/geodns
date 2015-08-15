package main

import (
	"math/rand"

	"github.com/abh/geodns/Godeps/_workspace/src/github.com/miekg/dns"
)

func (label *Label) Picker(qtype uint16, max int, location *Location) Records {

	if qtype == dns.TypeANY {
		var result []Record
		for rtype := range label.Records {

			rtypeRecords := label.Picker(rtype, max, location)

			tmpResult := make(Records, len(result)+len(rtypeRecords))

			copy(tmpResult, result)
			copy(tmpResult[len(result):], rtypeRecords)
			result = tmpResult
		}

		return result
	}

	if labelRR := label.Records[qtype]; labelRR != nil {

		// not "balanced", just return all
		if label.Weight[qtype] == 0 {
			return labelRR
		}

		rrCount := len(labelRR)
		if max > rrCount {
			max = rrCount
		}

		servers := make([]Record, len(labelRR))
		copy(servers, labelRR)
		result := make([]Record, max)
		sum := label.Weight[qtype]

		// Find the distance to each server, and find the servers that are
		// closer to the querier than the max'th furthest server, or within
		// 5% thereof. What this means in practice is that if we have a nearby
		// cluster of servers that are close, they all get included, so load
		// balancing works
		if qtype == dns.TypeA && location != nil && max < rrCount {
			// First we record the distance to each server
			distances := make([]float64, rrCount)
			for i, s := range servers {
				distance := location.Distance(s.Loc)
				distances[i] = distance
			}

			// though this looks like O(n^2), typically max is small (e.g. 2)
			// servers often have the same geographic location
			// and rrCount is pretty small too, so the gain of an
			// O(n log n) sort is small.
			chosen := 0
			choose := make([]bool, rrCount)

			for chosen < max {
				// Determine the minimum distance of servers not yet chosen
				minDist := location.MaxDistance()
				for i, _ := range servers {
					if !choose[i] && distances[i] <= minDist {
						minDist = distances[i]
					}
				}
				// The threshold for inclusion on the this pass is 5% more
				// than the minimum distance
				minDist = minDist * 1.05
				// Choose all the servers within the distance
				for i := range servers {
					if !choose[i] && distances[i] <= minDist {
						choose[i] = true
						chosen++
					}
				}
			}

			// Now choose only the chosen servers, using filtering without allocation
			// slice trick. Meanwhile recalculate the total weight
			tmpServers := servers[:0]
			sum = 0
			for i, s := range servers {
				if choose[i] {
					tmpServers = append(tmpServers, s)
					sum += s.Weight
				}
			}
			servers = tmpServers
		}

		for si := 0; si < max; si++ {
			n := rand.Intn(sum + 1)
			s := 0

			for i := range servers {
				s += int(servers[i].Weight)
				if s >= n {
					sum -= servers[i].Weight
					result[si] = servers[i]

					// remove the server from the list
					servers = append(servers[:i], servers[i+1:]...)
					break
				}
			}
		}

		return result
	}
	return nil
}
