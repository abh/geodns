# GeoDNS Changelog

## 2.2.0

- The CNAME configuration changed so the name of the current zone is appended
to the target name if the target is not a fqdn (ends with a "."). This is a rare
change not compatible with existing data. To upgrade make all cname's fqdn's until
all servers are running v2.2.0 or newer.
