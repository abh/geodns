# GeoDNS Changelog

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
