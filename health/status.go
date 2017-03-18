package health

import (
	"fmt"
	"strings"
	"sync"
)

// todo: how to deal with multiple files?
// specified in zone and a status object for each?

type StatusType uint8

const (
	StatusUnknown StatusType = iota
	StatusUnhealthy
	StatusHealthy
)

type Status interface {
	GetStatus(string) StatusType
	Reload() error
	Close() error
}

type statusRegistry struct {
	mu sync.RWMutex
	m  map[string]Status
}

var registry statusRegistry

type Service struct {
	Status StatusType
}

func init() {
	registry = statusRegistry{
		m: make(map[string]Status),
	}
}

func (r *statusRegistry) Add(name string, status Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[name] = status
	return nil
}

func (st StatusType) String() string {
	switch st {
	case StatusHealthy:
		return "healthy"
	case StatusUnhealthy:
		return "unhealthy"
	case StatusUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("status=%d", st)
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
