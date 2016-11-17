all: templates.go
	go build

templates.go: templates/*.html monitor.go
	go generate

test:
	go test -race $(go list ./... | grep -v /vendor/)

devel:
	go build -tags devel

bench:
	go test -check.b -check.bmem
