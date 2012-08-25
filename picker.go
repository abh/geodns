package main

import (
	"fmt"
)

func (label *Label) Picker(dnsType uint16, max int) Records {

	if label_rr := label.Records[dnsType]; label_rr != nil {

		//fmt.Printf("REGION_RR %T %v\n", label_rr, label_rr)

		// not "balanced", just return all
		if label.Weight[dnsType] == 0 {
			return label_rr
		}

		rr_count := len(label_rr)
		if max > rr_count {
			max = rr_count
		}

		fmt.Println("Total weight", label.Weight[dnsType])

		// TODO(ask) Pick random servers based on weight, not just the first 'max' entries
		servers := label_rr[0:max]

		fmt.Println("SERVERS", servers)

		return servers
	}
	return nil
}
