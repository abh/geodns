
all: status.html.go

status.html.go: templates/*.html
	go-bindata  -o status.html.go.tmp templates/
	@echo "// +build !devel\n" > status.html.go
	@cat status.html.go.tmp >> status.html.go
	@rm status.html.go.tmp
