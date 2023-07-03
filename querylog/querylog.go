package querylog

type QueryLogger interface {
	Write(*Entry) error
	Close() error
}

type Entry struct {
	Time       int64
	Hostname   string `json:",omitempty"` // not filled in by geodns
	Origin     string
	Name       string
	Qtype      uint16
	Rcode      int
	Answers    int
	Targets    []string
	AnswerData []string
	LabelName  string
	RemoteAddr string
	ClientAddr string
	HasECS     bool
	IsTCP      bool
	Version    string

	// todo:
	// - GeoDNS version
	// - TCP?
	// - log the answer data
}
