# GeoDNS in Go

This is the DNS server powering the [NTP Pool](http://www.pool.ntp.org/) system and other
similar services. It supersedes the [pgeodns](http://github.com/abh/pgeodns) server.
[![Build Status](https://drone.io/github.com/abh/geodns/status.png)](https://drone.io/github.com/abh/geodns/latest)

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
./geodns -h
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

After building the server you can run it with:

`./geodns -log -interface 127.1 -port 5053`

To test the responses run

`dig -t a test.example.com @127.1 -p 5053`

The binary can be moved to /usr/local/bin, /opt/geodns/ or wherever you find appropriate.

### Command options

Notable command line parameters (and their defaults)

* -config="./dns/"

Directory of zone files (and configuration named `geodns.conf`).

* -checkconfig=false

Check configuration file, parse zone files and exit

* -interface="*"

Comma separated IPs to listen on for DNS requests.

* -port="53"

Port number for DNS requests (UDP and TCP)

* -http=":8053"

Listen address for HTTP interface. Specify as `127.0.0.1:8053` to only listen on
localhost.

* -identifier=""

Identifier for this instance (hostname, pop name or similar).

It can also be a comma separated list of identifiers where the first is the "server id"
and subsequent ones are "group names", for example region of the server, name of anycast
cluster the server is part of, etc. This is used in (future) reporting/statistics features.

* -log=false

Enable to get lots of extra logging, only useful for testing and debugging. Absolutely not
recommended in production unless you get very few queries (less than 1-200/second).

* -cpus=1

Maximum number of CPUs to use. Set to 0 to match the number of CPUs available on the system.
Only "1" (the default) has been extensively tested.

## WebSocket interface

geodns runs a WebSocket server on port 8053 that outputs various performance
metrics. The WebSocket URL is `/monitor`. There's a "companion program" that can
use this across a cluster to show aggregate statistics, email for more information.

## Runtime status

There's a page with various runtime information (queries per second, queries and
most frequently requested labels per zone, etc) at `/status`.

## StatHat integration

GeoDNS can post runtime data to [StatHat](http://www.stathat.com/).
([Documentation](https://github.com/abh/geodns/wiki/StatHat))

## Country and continent lookups

## Weighted records

Most records can have a 'weight' assigned. If any records of a particular type
for a particular name have a weight, the system will return `max_hosts` records
(default 2).

If the weight for all records is 0, all matching records will be returned. The
weight for a label can be any integer as long as the weights for a label and record
type is less than 2 billion.

As an example, if you configure

    10.0.0.1, weight 10
    10.0.0.2, weight 20
    10.0.0.3, weight 30
    10.0.0.4, weight 40

with `max_hosts` 2 then .4 will be returned about 4 times more often than .1.

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
            "": {
                "ns": [ "ns.example.net", "ns2.example.net" ],
                "txt": "Example zone",
                "mx": { "mx": "mail.example.com", "preference": 10 }
            },
            "mail": { "a": [ ["192.168.0.1", 100], ["192.168.10.1", 50] ] },
            "mail.europe": { "a": [ ["192.168.255.1", 0] ] },
            "smtp": { "alias": "mail" }
        }
    }

The configuration files are automatically reloaded when they're updated. If a file
can't be read (invalid JSON, for example) the previous configuration for that zone
will be kept.

## Supported record types

Each label has a hash (object/associative array) of record data, the keys are the type.
The supported types and their options are listed below.

Adding support for more record types is relatively straight forward, please open a
ticket in the issue tracker with what you are missing.

### A

Each record has the format of a short array with the first element being the
IP address and the second the weight.

    [ [ "192.168.0.1", 10], ["192.168.2.1", 5] ]

See above for how the weights work.

### AAAA

Same format as A records (except the record type is "aaaa").

### Alias

Internally resolved cname, of sorts. Only works internally in a zone.

    "foo"

### CNAME

    "target.example.com."
    "www"

The target will have the current zone name appended if it's not a FQDN (since v2.2.0).

### MX

MX records support a `weight` similar to A records to indicate how often the particular
record should be returned.

The `preference` is the MX record preference returned to the client.

    { "mx": "foo.example.com" }
    { "mx": "foo.example.com", "weight": 100 }
    { "mx": "foo.example.com", "weight": 100, "preference": 10 }

`weight` and `preference` are optional.

### NS

NS records for the label, use it on the top level empty label (`""`) to specify
the nameservers for the domain.

    [ "ns1.example.com", "ns2.example.com" ]

There's an alternate legacy syntax that has space for glue records (IPv4 addresses),
but in GeoDNS the values in the object are ignored so the list syntax above is
recommended.

    { "ns1.example.net.": null, "ns2.example.net.": null }

### TXT

Simple syntax

    "Some text"

Or with weights

    { "txt": "Some text", "weight": 10 }

## License and Copyright

This software is Copyright 2012-2013 Ask Bjørn Hansen. For licensing information
please see the file called LICENSE.
