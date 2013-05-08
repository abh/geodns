package main

import (
	"github.com/abh/dns"
	"math/rand"
)

func (label *Label) Picker(qtype uint16, max int) Records {

	if qtype == dns.TypeANY {
		result := make([]Record, 0)
		for rtype := range label.Records {

			rtype_records := label.Picker(rtype, max)

			tmp_result := make(Records, len(result)+len(rtype_records))

			copy(tmp_result, result)
			copy(tmp_result[len(result):], rtype_records)
			result = tmp_result
		}

		return result
	}

	if label_rr := label.Records[qtype]; label_rr != nil {

		// not "balanced", just return all
		if label.Weight[qtype] == 0 {
			return label_rr
		}

		rr_count := len(label_rr)
		if max > rr_count {
			max = rr_count
		}

		servers := make([]Record, len(label_rr))
		copy(servers, label_rr)
		result := make([]Record, max)
		sum := label.Weight[qtype]

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
