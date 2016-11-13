package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
	"gopkg.in/gcfg.v1"
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
	HTTP struct {
		User     string
		Password string
	}
	QueryLog struct {
		Path    string
		MaxSize int
		Keep    int
	}
}

var Config = new(AppConfig)
var cfgMutex sync.RWMutex

func (conf *AppConfig) HasStatHat() bool {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return conf.Flags.HasStatHat
}

func (conf *AppConfig) StatHatApiKey() string {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return conf.StatHat.ApiKey
}

func (conf *AppConfig) GeoIPDirectory() string {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	return conf.GeoIP.Directory
}

func configWatcher(fileName string) {

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
			log.Println("fsnotify error:", err)
		}
	}

}

var lastReadConfig time.Time

func configReader(fileName string) error {

	stat, err := os.Stat(fileName)
	if err != nil {
		log.Printf("Failed to find config file: %s\n", err)
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
		log.Printf("Failed to parse config data: %s\n", err)
		return err
	}

	cfg.Flags.HasStatHat = len(cfg.StatHat.ApiKey) > 0

	// log.Println("STATHAT APIKEY:", cfg.StatHat.ApiKey)
	// log.Println("STATHAT FLAG  :", cfg.Flags.HasStatHat)

	cfgMutex.Lock()
	*Config = *cfg // shallow copy to prevent race conditions in referring to Config.foo()
	cfgMutex.Unlock()

	return nil
}
