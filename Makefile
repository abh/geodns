all: templates.go
	go build

templates.go: templates/*.html monitor.go
	go generate

devel:
	go build -tags devel
