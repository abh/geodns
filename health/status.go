package health

import (
	"strings"
	"sync"
)

// todo: how to deal with multiple files?
// specified in zone and a status object for each?

type StatusType uint8

const (
	StatusUnhealthy StatusType = iota
	StatusHealthy
	StatusUnknown
)

type Status interface {
	// Load(string) error
	GetStatus(string) StatusType
}

type statusRegistry struct {
	mu sync.RWMutex
	m  map[string]Status
}

var registry statusRegistry

type Service struct {
	Status StatusType
}

type StatusFile struct {
	mu sync.RWMutex
	m  map[string]*Service
}

func init() {
	registry = statusRegistry{
		m: make(map[string]Status),
	}
}

func GetStatus(name string) StatusType {
	check := strings.SplitN(name, "/", 2)
	if len(check) != 2 {
		return StatusUnknown
	}
	registry.mu.RLock()
	status, ok := registry.m[check[0]]
	registry.mu.RUnlock()

	if !ok {
		return StatusUnknown
	}
	return status.GetStatus(check[1])
}

func NewStatusFile() *StatusFile {
	return &StatusFile{
		m: make(map[string]*Service),
	}
}

func (s *StatusFile) Load(filename string) error {
	return nil
}

func (s *StatusFile) GetStatus(check string) StatusType {
	return StatusUnknown
}
