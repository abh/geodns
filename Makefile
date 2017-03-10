all: templates.go
	./build

templates.go: templates/*.html monitor.go
	go generate

test: .PHONY
	go test -v $(shell go list ./... | grep -v /vendor/)

testrace: .PHONY
	go test -v -race $(shell go list ./... | grep -v /vendor/)

docker-test: .PHONY
	mkdir -p .cache/pkg
	docker run --rm -v `pwd`:/go/src/github.com/abh/geodns \
		-v `pwd`/.cache/pkg:/go/pkg \
		geodns-build \
		make test
		# go test -i ./...

devel:
	go build -tags devel

bench:
	go test -check.b -check.bmem

TARS=$(wildcard geodns-*-*.tar)

push: $(TARS) tmp-install.sh
	rsync -avz tmp-install.sh $(TARS)  x3.dev:webtmp/2016/07/

builds: linux-build linux-build-i386 freebsd-build push

linux-build:
	docker run --rm -v `pwd`:/go/src/github.com/abh/geodns geodns-build ./build

linux-build-i386:
	docker run --rm -v `pwd`:/go/src/github.com/abh/geodns geodns-build-i386 ./build

freebsd-build:
	ssh 192.168.64.5 'cd go/src/github.com/abh/geodns; GOPATH=~/go ./build'
	ssh root@192.168.64.5 'jexec -U ask fbsd32 /home/ask/build'

.PHONY:
