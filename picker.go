package main

import (
	"math/rand"

	"github.com/miekg/dns"
)

func (label *Label) Picker(qtype uint16, max int) Records {

	if qtype == dns.TypeANY {
		var result []Record
		for rtype := range label.Records {

			rtypeRecords := label.Picker(rtype, max)

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

		if qtype == dns.TypeCNAME || qtype == dns.TypeMF {
			max = 1
		}

		rrCount := len(labelRR)
		if max > rrCount {
			max = rrCount
		}

		servers := make([]Record, len(labelRR))
		copy(servers, labelRR)
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
