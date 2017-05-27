package health

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"sync"
	"time"
)

type StatusFile struct {
	filename string
	mu       sync.RWMutex
	m        StatusFileData
}

type StatusFileData map[string]*Service

func NewStatusFile(filename string) *StatusFile {
	return &StatusFile{
		m:        make(StatusFileData),
		filename: filename,
	}
}

// DirectoryReader loads (and regularly re-loads) health
// .json files from the specified files into the default
// health registry.
func DirectoryReader(dir string) {
	for {
		err := reloadDirectory(dir)
		if err != nil {
			log.Printf("loading health data: %s", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func reloadDirectory(dir string) error {
	dirlist, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read '%s': %s", dir, err)
	}

	seen := map[string]bool{}

	var parseErr error

	for _, file := range dirlist {
		fileName := file.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".json") ||
			strings.HasPrefix(path.Base(fileName), ".") ||
			file.IsDir() {
			continue
		}
		statusName := fileName[0:strings.LastIndex(fileName, ".")]

		registry.mu.Lock()
		s, ok := registry.m[statusName]
		registry.mu.Unlock()

		seen[statusName] = true

		if ok {
			s.Reload()
		} else {
			s := NewStatusFile(path.Join(dir, fileName))
			err := s.Reload()
			if err != nil {
				log.Printf("error loading '%s': %s", fileName, err)
				parseErr = err
			}
			registry.Add(statusName, s)
		}
	}

	registry.mu.Lock()
	for n, _ := range registry.m {
		if !seen[n] {
			registry.m[n].Close()
			delete(registry.m, n)
		}
	}
	registry.mu.Unlock()

	return parseErr
}

func (s *StatusFile) Reload() error {
	if len(s.filename) > 0 {
		return s.Load(s.filename)
	}
	return nil
}

// Load imports the data atomically into the status map. If there's
// a JSON error the old data is preserved.
func (s *StatusFile) Load(filename string) error {
	n := StatusFileData{}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &n)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.m = n
	s.mu.Unlock()

	return nil
}

func (s *StatusFile) Close() error {
	s.mu.Lock()
	s.m = nil
	s.mu.Unlock()
	return nil
}

func (s *StatusFile) GetStatus(check string) StatusType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.m == nil {
		return StatusUnknown
	}

	st, ok := s.m[check]
	if !ok {
		log.Printf("Not found '%s'", check)
		return StatusUnknown
	}
	return st.Status
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (srv *Service) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	*srv = Service{Status: StatusType(i)}
	return nil
}

// UnmarshalJSON implements the json.Marshaler interface.
// func (srv *Service) MarshalJSON() ([]byte, error) {
// return
// }
