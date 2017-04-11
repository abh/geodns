package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/gcfg.v1"
	"gopkg.in/fsnotify.v1"
)

type AppConfig struct {
	StatHat struct {
		ApiKey string
	}
	Flags struct {
		HasStatHat bool
	}
	GeoIP struct {
		Directory string
	}
	StatsD struct {
		Host              string
		Port              int
		IntervalInSeconds int
	}
}

var Config = new(AppConfig)

func configWatcher(fileName string) {

	configReader(fileName)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println(err)
		return
	}

	if err := watcher.Add(*flagconfig); err != nil {
		fmt.Println(err)
		return
	}

	for {
		select {
		case ev := <-watcher.Events:
			if ev.Name == fileName {
				// Write = when the file is updated directly
				// Rename = when it's updated atomicly
				// Chmod = for `touch`
				if ev.Op&fsnotify.Write == fsnotify.Write ||
					ev.Op&fsnotify.Rename == fsnotify.Rename ||
					ev.Op&fsnotify.Chmod == fsnotify.Chmod {
					time.Sleep(200 * time.Millisecond)
					configReader(fileName)
				}
			}
		case err := <-watcher.Errors:
			logError("fsnotify error:", err)
		}
	}

}

var lastReadConfig time.Time

func configReader(fileName string) error {

	stat, err := os.Stat(fileName)
	if err != nil {
		logError("Failed to find config file", err)
		return err
	}

	if !stat.ModTime().After(lastReadConfig) {
		return err
	}

	lastReadConfig = time.Now()

	log.Printf("Loading config: %s\n", fileName)

	cfg := new(AppConfig)

	err = gcfg.ReadFileInto(cfg, fileName)
	if err != nil {
		logError("Failed to parse config data", err)
		return err
	}

	cfg.Flags.HasStatHat = len(cfg.StatHat.ApiKey) > 0

	// log.Println("STATHAT APIKEY:", cfg.StatHat.ApiKey)
	// log.Println("STATHAT FLAG  :", cfg.Flags.HasStatHat)

	Config = cfg

	return nil
}
