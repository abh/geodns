
all: status.html.go

status.html.go: status.html
	go-bindata  -i status.html -o status.html.go -p main -f status_html

