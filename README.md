# GeoDNS in Go

This is a (so far) experimental Golang implementation of the
[pgeodns](http://github.com/abh/pgeodns) server powering the [NTP
Pool](http://www.pool.ntp.org/) system.

## Installation

Run `go get` to install the Go dependencies. You will also need the
GeoIP C library.

## Sample configuration

Download some sample configuration files:

```sh
mkdir dns
curl -o dns/ntppool.org.json http://tmp.askask.com/2012/08/dns/ntppool.org.json.big
curl -o dns/example.com.json http://tmp.askask.com/2012/08/dns/example.com.json
```

## Run it

`go run *.go -log -run`

To test the responses run

`dig -t a ntppool.org  @127.1 -p 8053`
