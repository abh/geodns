# GeoDNS Changelog

## 2.2.2, February 2013

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
