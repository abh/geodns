module github.com/abh/geodns

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/abh/errorutil v0.0.0-20130729183701-f9bd360d00b9
	github.com/fsnotify/fsnotify v1.4.7
	github.com/golang/geo v0.0.0-20190916061304-5b978397cfec
	github.com/google/uuid v1.1.1 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/miekg/dns v1.1.26
	github.com/nxadm/tail v1.4.4
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/pborman/uuid v1.2.0
	github.com/prometheus/client_golang v1.3.0
	github.com/stretchr/testify v1.4.1-0.20191223143401-858f37ff9bc4
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876 // indirect
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553 // indirect
	golang.org/x/sys v0.0.0-20191228213918-04cbcbbfeed8 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/natefinch/lumberjack.v2 v2.0.1-0.20190411184413-94d9e492cc53
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
)

replace github.com/miekg/dns v1.1.26 => github.com/abh/dns v1.1.26-1
