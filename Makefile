all: templates.go
	./build

templates.go: templates/*.html monitor.go
	go generate

test: .PHONY
	go test -v $(shell go list ./... | grep -v /vendor/)

testrace: .PHONY
	go test -v -race $(shell go list ./... | grep -v /vendor/)

docker-test: .PHONY
	# test that we don't have missing dependencies
	docker run --rm -v `pwd`:/go/src/github.com/abh/geodns \
		-v /opt/local/share/GeoIP:/opt/local/share/GeoIP \
		golang:1.10-alpine3.7 \
		go test ./...

devel:
	go build -tags devel

bench:
	go test -check.b -check.bmem

TARS=$(wildcard geodns-*-*.tar)

push: $(TARS) tmp-install.sh
	rsync -avz tmp-install.sh $(TARS)  x3.dev:webtmp/2018/02/

builds: linux-build linux-build-i386 freebsd-build push

linux-build:
	GOOS=linux GOARCH=amd64 ./build

linux-build-i386:
	GOOS=linux GOARCH=386 ./build

freebsd-build:
	GOOS=freebsd GOARCH=amd64 ./build
	GOOS=freebsd GOARCH=386 ./build

.PHONY:
