all: templates.go
	./build

templates.go: templates/*.html monitor.go
	go generate

test:
	go test $(go list ./... | grep -v /vendor/)

testrace:
	go test -race $(go list ./... | grep -v /vendor/)

devel:
	go build -tags devel

bench:
	go test -check.b -check.bmem
