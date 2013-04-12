package main

import (
	"github.com/stathat/go"
	"log"
	"runtime"
	"strings"
	"time"
)

func statHatPoster() {

	lastQueryCount := qCounter.Count()
	stathatGroups := append(serverGroups, "total", serverId)
	suffix := strings.Join(stathatGroups, ",")
	// stathat.Verbose = true

	for {
		time.Sleep(60 * time.Second)

		if !Config.Flags.HasStatHat {
			log.Println("No stathat configuration")
			continue
		}

		log.Println("Posting to stathat")

		current := qCounter.Count()
		newQueries := current - lastQueryCount
		lastQueryCount = current

		stathat.PostEZCount("queries~"+suffix, Config.StatHat.ApiKey, int(newQueries))
		stathat.PostEZValue("goroutines "+serverId, Config.StatHat.ApiKey, float64(runtime.NumGoroutine()))

	}
}
