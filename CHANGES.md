# GeoDNS Changelog

## 2.4.4

* Fix parsing of 'targeting' option
* Add server id and ip to _country responses for easier debugging.

## 2.4.3

* Fix GeoIP custom directory bug (in geoip library)

## 2.4.2

* Update EDNS-SUBNET option number (in dns library)

## 2.4.1

* Update dns API to use new CountLabel and SplitDomainName functions
* Add test for mIXed-caSE queries (fix was in dns library)

## 2.4.0

* Add per-zone targeting configuration
* Support targeting by region/state with GeoIPCity
* Don't send backlogged zone counts to stathat when support is enabled

## 2.3.0, May 7th 2013
* Fix edns-client-subnet bug in dns library so it
  works with OpenDNS

## 2.2.8, April 28th 2013
* Support per-zone stats posted to StatHat
* Support TXT records
* Don't return NXDOMAIN for A queries to _status and _country
* Set serial number from file modtime if not explicitly set in json
* Improve record type documentation
* Warn about unknown record types in zone json files
* Add -version option

## 2.2.7, April 16th 2013
* Count EDNS queries per zone, pretty status page
  * Status page has various per-zone stats
  * Show global query stats, etc
* Add option to configure 'loggers'
* Add -cpus option to use multiple CPUs
* Add sample geodns.conf
* Use numbers instead of strings when appropriate in websocket stream
* Various refactoring and bug-fixes

## 2.2.6, April 9th 2013

* Begin more detailed /status page
* Make SOA record look more "normal" (cosmetic change only)

## 2.2.5, April 7th 2013

* Add StatHat feature
* Improve error handling for bad zone files
* Don't call runtime.GC() after loading each zone
* Set the minimum TTL to 10x regular TTL (up to an hour)
* service script: Load identifier from env/ID if it exists
* Work with latest geoip; use netmask from GeoIP in EDNS-SUBNET replies

## 2.2.4, March 5th 2013

* Add licensing information
* De-configure zones when the .json file is removed
* Start adding support for a proper configuration file
* Add -identifier command line option
* Various tweaks

## 2.2.3, March 1st 2013

* Always log when zones are re-read
* Remove one of the runtime.GC() calls when configs are loaded
* Set ulimit -n 64000 in run script
* Cleanup unused Zones variable in a few places
* Log when server was started to websocket /monitor interface

## 2.2.2, February 27th 2013

* Fix crash when getting unknown RRs in Extra request section

## 2.2.1, February 2013

* Beta EDNS-SUBNET support.
* Allow A/AAAA records without a weight
* Don't crash if a zone doesn't have any apex records
* Show line with syntax error when parsing JSON files
* Add --checkconfig parameter
* More tests


## 2.2.0, December 2012

* Initial EDNS-SUBNET support.
* Better error messages when parsing invalid JSON.
* -checkconfig command line option that loads the configuration and exits.
* The CNAME configuration changed so the name of the current zone is appended
  to the target name if the target is not a fqdn (ends with a "."). This is a
  rare change not compatible with existing data. To upgrade make all cname's
  fqdn's until all servers are running v2.2.0 or newer.
