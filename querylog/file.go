package querylog

import (
	"encoding/json"

	"gopkg.in/natefinch/lumberjack.v2"
)

type FileLogger struct {
	logger lumberjack.Logger
}

func NewFileLogger(filename string, maxsize int, keep int) (*FileLogger, error) {
	fl := &FileLogger{}
	fl.logger = lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxsize, // megabytes
		MaxBackups: keep,
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

func (l *FileLogger) Close() error {
	return l.logger.Close()
}
