# GeoDNS Changelog

## 3.1.0 - in development

* NSID support
* Support for DNS Cookies
* dnsflagday cleanups

## 3.0.2 December 2019

* Better test errors when geoip2 files aren't found
* Require Go 1.13 or later (just for build script for now)
* Add geodns-logs to Docker image
* Fix targeting tests (GeoIP data changed)
* Update dependencies

## 3.0.1 April 2019

* Added Prometheus metrics support
* Removed /monitor websocket interface
* Removed /status and /status.json pages
* Support "closest" matching (instead of geo/asn labels) for A and AAAA records (Alex Bligh)
* Support for GeoIP2 databases (including IPv6 data and ASN databases)
* "Pluggable" targeting data support
* Support for "health status" in an external file (not documented)
* Integrated health check support coming later (integrated work done by Alex Bligh, but not functional in this release - his branch on Github has/had it working)
* Remove minimum TTL for NS records (Alex Bligh)
* More/updated tests
* Don't let the server ID be 127.0.0.1
* Use 'dep' to manage dependencies
* Remove built-in InfluxDB support from the log processing tool

## 2.7.0 February 13, 2017

* Add support for PTR records (Florent AIDE)
* Test improvements (Alex Bligh)
* Update github.com/miekg/dns
* Update github.com/rcrowley/go-metrics
* Use vendor/ instead of godep
* Make query logging (globally) configurable
* Support base configuration file outside the zone config directory
* service: Read extra args from env/ARGS

## 2.6.0 October 4, 2015

Leif Johansson:
* Start new /status.json statistics end-point

Alex Bligh:
* Add ability to log to file.
* Add option to make debugging queries private.
* Fix race referencing config and other configuration system improvements.
* Fix crash on removal of zonefile with invalid JSON (Issue #69)
* Fix issue #74 - crash on reenabling previously invalid zone

Ask Bjørn Hansen:
* Fix critical data race in serve.go (and other rare races)
* Optionally require basic authentication for http interface
* Fix weighted CNAMEs (only return one)
* Make /status.json dump all metrics from go-metrics
* Update godeps (including miekg/dns)
* StatHat bugfix when the configuration changed at runtime
* ./build should just build, not install
* Fix crash when removing an invalid zone file
* Don't double timestamps when running under supervise
* Require Go 1.4+
* Internal improvements to metrics collection
* Remove every minute logging of goroutine and query count
* Add per-instance UUID to parsable status outputs (experimental)
* Report Go version as part of the version reporting
* Minor optimizations

## 2.5.0 June 5, 2015

* Add resolver ASN and IP targeting (Ewan Chou)
* Support for SPF records (Afsheen Bigdeli)
* Support weighted CNAME responses
* Add /48 subnet targeting for IPv6 ip targeting
* Don't log metrics to stderr anymore
* Make TTLs set on individual labels work
* Return NOERROR for "bar" if "foo.bar" exists (Geoffrey Papilion)
* Add Illinois to the us-central region group
* Add benchmark tests (Miek Gieben)
* Improve documentation
* Use godep to track code dependencies
* Don't add a '.' prefix on the record header on apex records

## 2.4.4 October 3, 2013

* Fix parsing of 'targeting' option
* Add server id and ip to _country responses for easier debugging.

## 2.4.3 October 1, 2013

* Fix GeoIP custom directory bug (in geoip library)

## 2.4.2 September 20, 2013

* Update EDNS-SUBNET option number (in dns library)

## 2.4.1 July 24, 2013

* Update dns API to use new CountLabel and SplitDomainName functions
* Add test for mIXed-caSE queries (fix was in dns library)

## 2.4.0 June 26, 2013

* Add per-zone targeting configuration
* Support targeting by region/state with GeoIPCity
* Don't send backlogged zone counts to stathat when support is enabled

## 2.3.0 May 7, 2013
* Fix edns-client-subnet bug in dns library so it
  works with OpenDNS

## 2.2.8 April 28, 2013
* Support per-zone stats posted to StatHat
* Support TXT records
* Don't return NXDOMAIN for A queries to _status and _country
* Set serial number from file modtime if not explicitly set in json
* Improve record type documentation
* Warn about unknown record types in zone json files
* Add -version option

## 2.2.7 April 16, 2013
* Count EDNS queries per zone, pretty status page
  * Status page has various per-zone stats
  * Show global query stats, etc
* Add option to configure 'loggers'
* Add -cpus option to use multiple CPUs
* Add sample geodns.conf
* Use numbers instead of strings when appropriate in websocket stream
* Various refactoring and bug-fixes

## 2.2.6 April 9, 2013

* Begin more detailed /status page
* Make SOA record look more "normal" (cosmetic change only)

## 2.2.5 April 7, 2013

* Add StatHat feature
* Improve error handling for bad zone files
* Don't call runtime.GC() after loading each zone
* Set the minimum TTL to 10x regular TTL (up to an hour)
* service script: Load identifier from env/ID if it exists
* Work with latest geoip; use netmask from GeoIP in EDNS-SUBNET replies

## 2.2.4 March 5, 2013

* Add licensing information
* De-configure zones when the .json file is removed
* Start adding support for a proper configuration file
* Add -identifier command line option
* Various tweaks

## 2.2.3 March 1, 2013

* Always log when zones are re-read
* Remove one of the runtime.GC() calls when configs are loaded
* Set ulimit -n 64000 in run script
* Cleanup unused Zones variable in a few places
* Log when server was started to websocket /monitor interface

## 2.2.2 February 27, 2013

* Fix crash when getting unknown RRs in Extra request section

## 2.2.1 February 2013

* Beta EDNS-SUBNET support.
* Allow A/AAAA records without a weight
* Don't crash if a zone doesn't have any apex records
* Show line with syntax error when parsing JSON files
* Add --checkconfig parameter
* More tests


## 2.2.0 December 2012

* Initial EDNS-SUBNET support.
* Better error messages when parsing invalid JSON.
* -checkconfig command line option that loads the configuration and exits.
* The CNAME configuration changed so the name of the current zone is appended
  to the target name if the target is not a fqdn (ends with a "."). This is a
  rare change not compatible with existing data. To upgrade make all cname's
  fqdn's until all servers are running v2.2.0 or newer.
