# GeoDNS in Go

This is a (so far) experimental Golang implementation of the
[pgeodns](http://github.com/abh/pgeodns) server powering the [NTP
Pool](http://www.pool.ntp.org/) system.
[![Build Status](https://secure.travis-ci.org/abh/geodns.png)](http://travis-ci.org/abh/geodns)

## Installation

If you already have go installed, just run `go get` to install the Go dependencies.

You will also need the GeoIP C library, on RedHat derived systems
that's `yum install geoip-devel`.

If you don't have Go installed the easiest way to build geodns from source is to
download Go from http://code.google.com/p/go/downloads/list and untar'ing it in
`/usr/local/go` and then run the following from a regular user account:

```sh
export PATH=$PATH:/usr/local/go/bin
export GOPATH=~/go
go get github.com/abh/geodns
cd ~/go/src/github.com/abh/geodns
go test
go build
```

## Sample configuration

There's a sample configuration file in `dns/example.com.json`. This is currently
derived from the `test.example.com` data used for unit tests and not an example
of a "best practices" configuration.

For testing there's also a bigger test file at:

```sh
mkdir -p dns
curl -o dns/test.ntppool.org.json http://tmp.askask.com/2012/08/dns/ntppool.org.json.big
```

## Run it

`go run *.go -log -interface 127.1 -port 5053`

or if you already built geodns, then `./geodns ...`.

To test the responses run

`dig -t a test.example.com @127.1 -p 5053`

## WebSocket interface

geodns runs a WebSocket server on port 8053 that outputs various performance
metrics, see `monitor.go` for details.

## Country and continent lookups

## Weighted records

Except for NS records all records can have a 'weight' assigned. If any records
of a particular type for a particular name have a weight, the system will return
the "max_hosts" records (default 2)

## Configuration format

In the configuration file the whole zone is a big hash (associative array). At the
top level you can (optionally) set some options with the keys serial, ttl and max_hosts.

The actual zone data (dns records) is in a hash under the key "data". The keys
in the hash are hostnames and the value for each hostname is yet another hash
where the keys are record types (lowercase) and the values an array of records.

For example to setup an MX record at the zone apex and then have a different
A record for users in Europe than anywhere else, use:

    {
        "serial": 1,
        "data": {
            "": { "mx": { "mx": "mail.example.com", "preference": 10 } },
            "mail": { "a": [ ["192.168.0.1", 100], ["192.168.10.1", 50] ] },
            "mail.europe": { "a": [ ["192.168.255.1", 0] ] },
            "smtp": { "alias": "mail" }
        }
    }

The configuration files are automatically reloaded when they're updated. If a file
can't be read (invalid JSON, for example) the previous configuration for that zone
will be kept.

## Supported record types

### A

Each record has the format of a short array with the first element being the
IP address and the second the weight.

   { "a": [ [ "192.168.0.1", 10], ["192.168.2.1", 5] ] }

### AAAA

Same format as A records (except the record type is "aaaa").

### CNAME

The target will have the current zone name appended if it's not a FQDN (since v2.2.0).

  { "cname": "target.example.com." }
  { "cname": "www" }

### NS

### MX

   { "mx": "foo.example.com" }

### Alias

Internally resolved cname, of sorts. Only works internally in a zone.

   { "alias": "foo" }
