
all: status.html.go

status.html.go: status.html
	go-bindata  -i status.html -o status.html.go.tmp -p main -f status_html
	@echo "// +build !devel\n" > status.html.go
	@cat status.html.go.tmp >> status.html.go
	@rm status.html.go.tmp
