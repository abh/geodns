package appconfig

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/gcfg.v1"

	"github.com/abh/geodns/v3/targeting/geoip2"
)

type AppConfig struct {
	DNS struct {
		PublicDebugQueries bool
		DetailedMetrics    bool
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
	AvroLog struct {
		Path    string
		MaxSize int    // rotate files at this size
		MaxTime string // rotate active files after this time, even if small
	}
	Health struct {
		Directory string
	}
	Nodeping struct {
		Token string
	}
	Pingdom struct {
		Username string

		Password     string
		AccountEmail string
		AppKey       string
		StateMap     string
	}
}

// Singleton to keep the latest read config
var Config = new(AppConfig)

var cfgMutex sync.RWMutex

func (conf *AppConfig) GeoIPDirectory() string {
	cfgMutex.RLock()
	defer cfgMutex.RUnlock()
	if len(conf.GeoIP.Directory) > 0 {
		return conf.GeoIP.Directory
	}
	return geoip2.FindDB()
}

func ConfigWatcher(ctx context.Context, fileName string) error {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(fileName); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-watcher.Events:
			if ev.Name == fileName {
				// Write = when the file is updated directly
				// Rename = when it's updated atomicly
				// Chmod = for `touch`
				if ev.Has(fsnotify.Write) ||
					ev.Has(fsnotify.Rename) ||
					ev.Has(fsnotify.Chmod) {
					time.Sleep(200 * time.Millisecond)
					err := ConfigReader(fileName)
					if err != nil {
						// don't quit because we'll just keep the old config at this
						// stage and try again next it changes
						log.Printf("error reading config file: %s", err)
					}
				}
			}
		case err := <-watcher.Errors:
			log.Printf("fsnotify error: %s", err)
		}
	}
}

var lastReadConfig time.Time

func ConfigReader(fileName string) error {

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

	cfgMutex.Lock()
	*Config = *cfg // shallow copy to prevent race conditions in referring to Config.foo()
	cfgMutex.Unlock()

	return nil
}
