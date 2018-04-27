package monitor

import (
	"time"
)

// ServerInfo has the configured ID and groups and the first IP
// address for the server among other 'who am I' information. The
// UUID is reset on each restart.
type ServerInfo struct {
	Version string
	ID      string
	IP      string
	UUID    string
	Groups  []string
	Started time.Time
}
