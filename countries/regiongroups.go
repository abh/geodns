package countries

import (
	"log"
)

var RegionGroups = map[string]string{
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

	// # Federal districts of Russia
	// Sources list (lowest priority on top)
	// - https://en.wikipedia.org/wiki/Federal_districts_of_Russia
	// - https://ru.wikipedia.org/wiki/ISO_3166-2:RU
	// - http://statoids.com/uru.html with updates https://en.wikipedia.org/wiki/Federal_districts_of_Russia#cite_note-15

	// Dal'nevostochnyy (D) Far Eastern
	"ru-amu": "ru-dfd", // Amur
	"ru-bu":  "ru-dfd", // Buryat
	"ru-chu": "ru-dfd", // Chukot
	"ru-kam": "ru-dfd", // Kamchatka
	"ru-kha": "ru-dfd", // Khabarovsk
	"ru-mag": "ru-dfd", // Magadan
	"ru-pri": "ru-dfd", // Primor'ye
	"ru-sa":  "ru-dfd", // Sakha
	"ru-sak": "ru-dfd", // Sakhalin
	"ru-yev": "ru-dfd", // Yevrey
	"ru-zab": "ru-dfd", // Zabaykal'ye

	// Severo-Kavkazskiy' (K) North Caucasus
	"ru-ce":  "ru-kfd", // Chechnya
	"ru-da":  "ru-kfd", // Dagestan
	"ru-in":  "ru-kfd", // Ingush
	"ru-kb":  "ru-kfd", // Kabardin-Balkar
	"ru-kc":  "ru-kfd", // Karachay-Cherkess
	"ru-se":  "ru-kfd", // North Ossetia
	"ru-sta": "ru-kfd", // Stavropol'

	// Privolzhskiy (P) Volga
	"ru-ba":  "ru-pfd", // Bashkortostan
	"ru-cu":  "ru-pfd", // Chuvash
	"ru-kir": "ru-pfd", // Kirov
	"ru-me":  "ru-pfd", // Mariy-El
	"ru-mo":  "ru-pfd", // Mordovia
	"ru-niz": "ru-pfd", // Nizhegorod
	"ru-ore": "ru-pfd", // Orenburg
	"ru-pnz": "ru-pfd", // Penza
	"ru-per": "ru-pfd", // Perm'
	"ru-sam": "ru-pfd", // Samara
	"ru-sar": "ru-pfd", // Saratov
	"ru-ta":  "ru-pfd", // Tatarstan
	"ru-ud":  "ru-pfd", // Udmurt
	"ru-uly": "ru-pfd", // Ul'yanovsk

	// Sibirskiy (S) Siberian
	"ru-alt": "ru-sfd", // Altay
	"ru-al":  "ru-sfd", // Gorno-Altay
	"ru-irk": "ru-sfd", // Irkutsk
	"ru-kem": "ru-sfd", // Kemerovo
	"ru-kk":  "ru-sfd", // Khakass
	"ru-kya": "ru-sfd", // Krasnoyarsk
	"ru-nvs": "ru-sfd", // Novosibirsk
	"ru-oms": "ru-sfd", // Omsk
	"ru-tom": "ru-sfd", // Tomsk
	"ru-ty":  "ru-sfd", // Tuva

	// Tsentral'nyy (T) Central
	"ru-bel": "ru-tfd", // Belgorod
	"ru-bry": "ru-tfd", // Bryansk
	"ru-iva": "ru-tfd", // Ivanovo
	"ru-klu": "ru-tfd", // Kaluga
	"ru-kos": "ru-tfd", // Kostroma
	"ru-krs": "ru-tfd", // Kursk
	"ru-lip": "ru-tfd", // Lipetsk
	"ru-mow": "ru-tfd", // Moscow City
	"ru-mos": "ru-tfd", // Moskva
	"ru-orl": "ru-tfd", // Orel
	"ru-rya": "ru-tfd", // Ryazan'
	"ru-smo": "ru-tfd", // Smolensk
	"ru-tam": "ru-tfd", // Tambov
	"ru-tul": "ru-tfd", // Tula
	"ru-tve": "ru-tfd", // Tver'
	"ru-vla": "ru-tfd", // Vladimir
	"ru-vor": "ru-tfd", // Voronezh
	"ru-yar": "ru-tfd", // Yaroslavl'

	// Ural'skiy (U) Ural
	"ru-che": "ru-ufd", // Chelyabinsk
	"ru-khm": "ru-ufd", // Khanty-Mansiy
	"ru-kgn": "ru-ufd", // Kurgan
	"ru-sve": "ru-ufd", // Sverdlovsk
	"ru-tyu": "ru-ufd", // Tyumen'
	"ru-yan": "ru-ufd", // Yamal-Nenets

	// Severo-Zapadnyy (V) Northwestern
	"ru-ark": "ru-vfd", // Arkhangel'sk
	"ru-kgd": "ru-vfd", // Kaliningrad
	"ru-kr":  "ru-vfd", // Karelia
	"ru-ko":  "ru-vfd", // Komi
	"ru-len": "ru-vfd", // Leningrad
	"ru-mur": "ru-vfd", // Murmansk
	"ru-nen": "ru-vfd", // Nenets
	"ru-ngr": "ru-vfd", // Novgorod
	"ru-psk": "ru-vfd", // Pskov
	"ru-spe": "ru-vfd", // Saint Petersburg City
	"ru-vlg": "ru-vfd", // Vologda

	// Yuzhnyy (Y) Southern
	"ru-ad":  "ru-yfd", // Adygey
	"ru-ast": "ru-yfd", // Astrakhan'
	"ru-kl":  "ru-yfd", // Kalmyk
	"ru-kda": "ru-yfd", // Krasnodar
	"ru-ros": "ru-yfd", // Rostov
	"ru-vgg": "ru-yfd", // Volgograd
}

var RegionGroupRegions = map[string][]string{}

func CountryRegionGroup(country, region string) string {

	if country != "us" && country != "ru" {
		return ""
	}

	if group, ok := RegionGroups[region]; ok {
		return group
	}

	log.Printf("Did not find a region group for '%s'/'%s'", country, region)
	return ""
}

func init() {
	for ccrc, rg := range RegionGroups {
		RegionGroupRegions[rg] = append(RegionGroupRegions[rg], ccrc)
	}
}
