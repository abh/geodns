package countries

import (
	"log"
)

func CountryRegionGroup(country, region string) string {

	if country != "us" {
		return ""
	}

	regions := map[string]string{
		"us-ak": "us-west",
		"us-az": "us-west",
		"us-ca": "us-west",
		"us-co": "us-west",
		"us-hi": "us-west",
		"us-id": "us-west",
		"us-mt": "us-west",
		"us-nm": "us-west",
		"us-nv": "us-west",
		"us-or": "us-west",
		"us-ut": "us-west",
		"us-wa": "us-west",
		"us-wy": "us-west",

		"us-ar": "us-central",
		"us-ia": "us-central",
		"us-il": "us-central",
		"us-in": "us-central",
		"us-ks": "us-central",
		"us-la": "us-central",
		"us-mn": "us-central",
		"us-mo": "us-central",
		"us-nd": "us-central",
		"us-ne": "us-central",
		"us-ok": "us-central",
		"us-sd": "us-central",
		"us-tx": "us-central",
		"us-wi": "us-central",

		"us-al": "us-east",
		"us-ct": "us-east",
		"us-dc": "us-east",
		"us-de": "us-east",
		"us-fl": "us-east",
		"us-ga": "us-east",
		"us-ky": "us-east",
		"us-ma": "us-east",
		"us-md": "us-east",
		"us-me": "us-east",
		"us-mi": "us-east",
		"us-ms": "us-east",
		"us-nc": "us-east",
		"us-nh": "us-east",
		"us-nj": "us-east",
		"us-ny": "us-east",
		"us-oh": "us-east",
		"us-pa": "us-east",
		"us-ri": "us-east",
		"us-sc": "us-east",
		"us-tn": "us-east",
		"us-va": "us-east",
		"us-vt": "us-east",
		"us-wv": "us-east",
	}

	if group, ok := regions[region]; ok {
		return group
	}

	log.Printf("Did not find a region group for '%s'/'%s'", country, region)
	return ""
}
