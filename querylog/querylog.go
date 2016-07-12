package querylog

import (
	"encoding/json"

	"gopkg.in/natefinch/lumberjack.v2"
)

type QueryLogger interface {
	Write(*Entry) error
}

// easyjson:json
type Entry struct {
	Time       int64
	Origin     string
	Name       string
	Qtype      uint16
	Rcode      int
	Answers    int
	Targets    []string
	LabelName  string
	RemoteAddr string
	ClientAddr string
	HasECS     bool
}

type FileLogger struct {
	logger lumberjack.Logger
}

func NewFileLogger(filename string) (*FileLogger, error) {
	fl := &FileLogger{}
	fl.logger = lumberjack.Logger{
		Filename:   filename,
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28, //days
	}
	return fl, nil
}

func (l *FileLogger) Write(e *Entry) error {
	js, err := json.Marshal(e)
	if err != nil {
		return err
	}
	js = append(js, []byte("\n")...)
	_, err = l.logger.Write(js)
	return err
}
