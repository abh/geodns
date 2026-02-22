# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GeoDNS is a geographic-aware DNS server powering the NTP Pool system. It returns different DNS responses based on the client's geographic location, supporting country, continent, region, and ASN-based targeting.

## Build and Test Commands

```bash
# Build
go build

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -v -run TestName ./path/to/package

# Run tests with race detector
go test -v -race ./...

# Check configuration without starting server
./geodns -checkconfig

# Run locally for testing
./geodns -log -interface 127.1 -port 5053

# Test with dig
dig -t a test.example.com @127.1 -p 5053
```

## Code Formatting

Always run `gofumpt -w` on modified `.go` files before committing.

## Architecture

### Main Components

- **geodns.go** - Entry point. Parses flags, initializes config, starts server and HTTP endpoints.
- **server/** - DNS server implementation using miekg/dns library.
  - `server.go` - Server struct, listener setup, Prometheus metrics registration.
  - `serve.go` - Request handling logic, EDNS client subnet processing, geo targeting.
- **zones/** - Zone data management.
  - `zone.go` - Zone and Label structs, DNS record storage, label lookup with geo fallback.
  - `reader.go` - JSON zone file parser. Supports A, AAAA, CNAME, MX, NS, TXT, SPF, SRV, PTR records.
  - `muxmanager.go` - Watches zone directory, auto-reloads on file changes (2-second poll).
  - `picker.go` - Weighted record selection algorithm.
- **targeting/** - Geographic targeting system.
  - `targeting.go` - Target options (global, continent, country, region, ASN, IP). Builds ordered target list for label lookups.
- **targeting/geoip2/** - MaxMind GeoIP2 database integration.
- **appconfig/** - Configuration file parsing (gcfg format). Watches for config changes.
- **health/** - Health check status tracking for records.
- **querylog/** - Query logging to JSON files or Avro format.

### Request Flow

1. DNS request arrives at `server.serve()`
2. EDNS client subnet extracted if present
3. `targeting.GetTargets()` returns ordered list: `[region, country, continent, @]`
4. `zone.FindLabels()` searches for matching label with fallback through targets
5. `zone.Picker()` selects records by weight
6. Response sent with appropriate EDNS scope

### Zone File Format

JSON files in `dns/` directory. Zone name derived from filename (e.g., `example.com.json`).

```json
{
  "ttl": 120,
  "max_hosts": 2,
  "targeting": "@ country continent",
  "data": {
    "": { "ns": ["ns1.example.com", "ns2.example.com"] },
    "www": { "a": [["192.0.2.1", 10], ["192.0.2.2", 20]] },
    "www.us": { "a": [["192.0.2.100", 0]] }
  }
}
```

Label naming: `label.target` (e.g., `www.us` for US-specific, `www.eu` for Europe).

### Configuration

`geodns.conf` in the zone directory. INI-style format with sections: `[dns]`, `[geoip]`, `[querylog]`, `[avrolog]`, `[http]`, `[health]`.

## Key Dependencies

- `codeberg.org/miekg/dns` - DNS protocol implementation (v2)
- `github.com/oschwald/geoip2-golang` - MaxMind GeoIP2 reader
- `github.com/prometheus/client_golang` - Prometheus metrics
- `github.com/hamba/avro/v2` - Avro query logging
