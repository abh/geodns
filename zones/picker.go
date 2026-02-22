package zones

import (
	"math/rand"

	"github.com/abh/geodns/v3/health"
	"github.com/abh/geodns/v3/targeting/geo"

	dnsv1 "github.com/miekg/dns"
)

// AlwaysWeighted types always honors 'MaxHosts', even
// without an explicit weight set. (This list is slightly arbitrary).
var AlwaysWeighted = []uint16{
	dnsv1.TypeA, dnsv1.TypeAAAA, dnsv1.TypeCNAME,
}

var alwaysWeighted map[uint16]struct{}

func init() {
	alwaysWeighted = map[uint16]struct{}{}
	for _, t := range AlwaysWeighted {
		alwaysWeighted[t] = struct{}{}
	}
}

func (zone *Zone) filterHealth(servers Records) (Records, int) {
	// Remove any unhealthy servers
	tmpServers := servers[:0]

	sum := 0
	for i, s := range servers {
		if len(servers[i].Test) == 0 || health.GetStatus(servers[i].Test) == health.StatusHealthy {
			tmpServers = append(tmpServers, s)
			sum += s.Weight
		}
	}
	return tmpServers, sum
}

// Picker picks the best results from a label matching the qtype,
// up to 'max' results. If location is specified Picker will get
// return the "closests" results, otherwise they are returned weighted
// randomized.
func (zone *Zone) Picker(label *Label, qtype uint16, max int, location *geo.Location) Records {
	if qtype == dnsv1.TypeANY {
		var result Records
		for rtype := range label.Records {

			rtypeRecords := zone.Picker(label, rtype, max, location)

			tmpResult := make(Records, len(result)+len(rtypeRecords))

			copy(tmpResult, result)
			copy(tmpResult[len(result):], rtypeRecords)
			result = tmpResult
		}

		return result
	}

	labelRR := label.Records[qtype]
	if labelRR == nil {
		// we don't have anything of the correct type
		return nil
	}

	sum := label.Weight[qtype]

	servers := make(Records, len(labelRR))
	copy(servers, labelRR)

	if label.Test != nil {
		servers, sum = zone.filterHealth(servers)
		// sum re-check to mirror the label.Weight[] check below
		if sum == 0 {
			// todo: this is wrong for cname since it misses
			// the 'max_hosts' setting
			return servers
		}
	}

	// not "balanced", just return all -- It's been working
	// this way since the first prototype, it might not make
	// sense anymore. This probably makes NS records and such
	// work as expected.
	// A, AAAA and CNAME records ("AlwaysWeighted") are always given
	// a weight so MaxHosts works for those even if weight isn't set.
	if label.Weight[qtype] == 0 {
		return servers
	}

	if qtype == dnsv1.TypeCNAME || qtype == dnsv1.TypeMF {
		max = 1
	}

	rrCount := len(servers)
	if max > rrCount {
		max = rrCount
	}
	result := make(Records, max)

	// Find the distance to each server, and find the servers that are
	// closer to the querier than the max'th furthest server, or within
	// 5% thereof. What this means in practice is that if we have a nearby
	// cluster of servers that are close, they all get included, so load
	// balancing works
	if location != nil && (qtype == dnsv1.TypeA || qtype == dnsv1.TypeAAAA) && max < rrCount {
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
			for i := range servers {
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
