package main

import (
	"log"
	"math/rand"
)

func (label *Label) Picker(dnsType uint16, max int) Records {

	if label_rr := label.Records[dnsType]; label_rr != nil {

		log.Printf("REGION_RR %i %T %v\n", len(label_rr), label_rr, label_rr)

		// not "balanced", just return all
		if label.Weight[dnsType] == 0 {
			return label_rr
		}

		rr_count := len(label_rr)
		if max > rr_count {
			max = rr_count
		}

		servers := make([]Record, len(label_rr))
		copy(servers, label_rr)
		result := make([]Record, max)
		sum := label.Weight[dnsType]

		for si := 0; si < max; si++ {
			n := rand.Intn(sum + 1)
			s := 0

			for i := range servers {
				s += int(servers[i].Weight)
				if s >= n {
					log.Println("Picked record", i, servers[i])
					sum -= servers[i].Weight
					result[si] = servers[i]

					// remove the server from the list
					servers = append(servers[:i], servers[i+1:]...)
					break
				}
			}
		}

		log.Println("SERVERS", result)

		return result
	}
	return nil
}
