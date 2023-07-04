package querylog

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"github.com/hamba/avro/v2"
	"github.com/hamba/avro/v2/ocf"
)

//go:embed querylog.avsc
var schemaJson string

type AvroLogger struct {
	path    string
	maxsize int
	maxtime time.Duration

	schema avro.Schema

	ctx    context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup

	ch chan *Entry
}

func NewAvroLogger(path string, maxsize int, maxtime time.Duration) (*AvroLogger, error) {

	schema, err := AvroSchema()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	l := &AvroLogger{
		ctx:     ctx,
		cancel:  cancel,
		path:    path,
		maxsize: maxsize,
		maxtime: maxtime,
		schema:  schema,
		ch:      make(chan *Entry, 2000),
		wg:      sync.WaitGroup{},
	}

	go l.writer(ctx)
	return l, nil
}

func AvroSchema() (avro.Schema, error) {
	schema, err := avro.Parse(schemaJson)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (l *AvroLogger) Write(e *Entry) error {
	select {
	case l.ch <- e:
		return nil
	default:
		return fmt.Errorf("buffer full")
	}
}

// func (l *AvroFile)

type avroFile struct {
	fh    *os.File
	enc   *ocf.Encoder
	open  bool
	count int
}

func (l *AvroLogger) writer(ctx context.Context) {

	mu := sync.Mutex{}

	log.Printf("opening")

	timer := time.After(l.maxtime)

	openFiles := []*avroFile{}

	openFile := func() (*avroFile, error) {
		// todo: communicate back to the main process when this goes wrong

		now := time.Now().UTC().Format("20060102-150405")

		f, err := os.CreateTemp(l.path, fmt.Sprintf("log.%s.*.avro.tmp", now))
		if err != nil {
			return nil, err
		}
		enc, err := ocf.NewEncoder(schemaJson, f, ocf.WithCodec(ocf.Snappy))

		if err != nil {
			return nil, err
		}

		l.wg.Add(1)
		a := &avroFile{fh: f, enc: enc, open: true}

		log.Printf("opened %s", a.fh.Name())

		mu.Lock()
		defer mu.Unlock()
		openFiles = append([]*avroFile{a}, openFiles...)

		timer = time.After(l.maxtime)

		return a, nil
	}

	log.Printf("setting up avro")

	currentFile, err := openFile()
	if err != nil {
		log.Fatalf("openfile error: %s", err)
	}

	closeFile := func(af *avroFile) error {

		mu.Lock()
		idx := slices.Index[*avroFile](openFiles, af)
		if idx >= 0 {
			openFiles = slices.Delete(openFiles, idx, idx+1)
		} else {
			log.Printf("could not find avroFile for closing in openFiles list")
		}

		if !af.open {
			log.Printf("called closeFile on file already being closed %s", af.fh.Name())
			return nil
		}

		af.open = false
		mu.Unlock()

		defer l.wg.Done()

		log.Printf("closing %s", af.fh.Name())

		if err := af.enc.Flush(); err != nil {
			return err
		}
		if err := af.fh.Sync(); err != nil {
			return err
		}
		if err := af.fh.Close(); err != nil {
			return err
		}

		tmpName := af.fh.Name()
		newName := strings.TrimSuffix(tmpName, ".tmp")
		if tmpName == newName {
			return fmt.Errorf("unexpected tmp file name %s", tmpName)
		}

		log.Printf("renaming to %s", newName)
		if err := os.Rename(tmpName, newName); err != nil {
			return err
		}
		return nil
	}

	for {
		select {
		case e := <-l.ch:
			currentFile.count++
			err := currentFile.enc.Encode(e)
			if err != nil {
				log.Fatal(err)
			}
			if currentFile.count%1000 == 0 {
				size, err := currentFile.fh.Seek(0, 2)
				if err != nil {
					log.Printf("could not seek avro file: %s", err)
					continue
				}
				if size > int64(l.maxsize) {
					log.Printf("rotating avro file for size")
					currentFile, err = openFile()
					if err != nil {
						log.Printf("could not open new avro file: %s", err)
					}
				}
			}

		case <-ctx.Done():
			log.Printf("context done, closing files")

			// drain the buffer within reason
			count := 0
		drain:
			for {
				select {
				case e := <-l.ch:
					count++
					err := currentFile.enc.Encode(e)
					if err != nil {
						log.Fatal(err)
					}
					if count > 10000 {
						break drain
					}
				default:
					break drain
				}
			}

			for i := len(openFiles) - 1; i >= 0; i-- {
				err := closeFile(openFiles[i])
				if err != nil {
					log.Printf("error closing file: %s", err)
				}
			}
			return

		case <-timer:
			if currentFile.count == 0 {
				timer = time.After(l.maxtime)
				continue
			}

			log.Printf("rotating avro file for time")

			var err error
			currentFile, err = openFile()
			if err != nil {
				log.Printf("could not open new avrofile: %s", err)
			} else {
				for i, af := range openFiles {
					if i == 0 || af == currentFile {
						continue
					}
					err := closeFile(af)
					if err != nil {
						log.Printf("error closing old avro files: %s", err)
					}
				}
			}
		}
	}

}

func (l *AvroLogger) Close() error {
	log.Printf("calling Close()")
	l.cancel(fmt.Errorf("closing"))
	<-l.ctx.Done()
	l.wg.Wait() // wait for all files to be closed
	log.Printf("Close() returning")
	return nil
}
