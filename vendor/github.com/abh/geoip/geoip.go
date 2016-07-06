/* Go (cgo) interface to libgeoip */
package geoip

/*
#cgo pkg-config: geoip  
#include <stdio.h>
#include <errno.h>
#include <GeoIP.h>
#include <GeoIPCity.h>

//typedef GeoIP* GeoIP_pnt
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"unsafe"
)

type GeoIP struct {
	db *C.GeoIP

	// We don't use GeoIP's thread-safe API calls, which means there is a
	// single global netmask variable that gets clobbered in the main
	// lookup routine.  Any calls which have _GeoIP_seek_record_gl need to
	// be wrapped in this mutex.

	mu sync.Mutex
}

func (gi *GeoIP) free() {
	if gi == nil {
		return
	}
	if gi.db == nil {
		gi = nil
		return
	}
	C.GeoIP_delete(gi.db)
	gi = nil
	return
}

// Default convenience wrapper around OpenDb
func Open(files ...string) (*GeoIP, error) {
	return OpenDb(files, GEOIP_MEMORY_CACHE)
}

// Opens a GeoIP database by filename with specified GeoIPOptions flag.
// All formats supported by libgeoip are supported though there are only
// functions to access some of the databases in this API.
// If you don't pass a filename, it will try opening the database from
// a list of common paths.
func OpenDb(files []string, flag int) (*GeoIP, error) {
	if len(files) == 0 {
		files = []string{
			"/usr/share/GeoIP/GeoIP.dat",       // Linux default
			"/usr/share/local/GeoIP/GeoIP.dat", // source install?
			"/usr/local/share/GeoIP/GeoIP.dat", // FreeBSD
			"/opt/local/share/GeoIP/GeoIP.dat", // MacPorts
			"/usr/share/GeoIP/GeoIP.dat",       // ArchLinux
		}
	}

	g := &GeoIP{}
	runtime.SetFinalizer(g, (*GeoIP).free)

	var err error

	for _, file := range files {

		// libgeoip prints errors if it can't open the file, so check first
		if _, err := os.Stat(file); err != nil {
			if os.IsExist(err) {
				log.Println(err)
			}
			continue
		}

		cbase := C.CString(file)
		defer C.free(unsafe.Pointer(cbase))

		g.db, err = C.GeoIP_open(cbase, C.int(flag))
		if g.db != nil && err != nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Error opening GeoIP database (%s): %s", files, err)
	}

	if g.db == nil {
		return nil, fmt.Errorf("Didn't open GeoIP database (%s)", files)
	}

	C.GeoIP_set_charset(g.db, C.GEOIP_CHARSET_UTF8)
	return g, nil
}

// SetCustomDirectory sets the default location for the GeoIP .dat files used when
// calling OpenType()
func SetCustomDirectory(dir string) {
	cdir := C.CString(dir)
	// GeoIP doesn't copy the string, so don't free it when we're done here.
	// defer C.free(unsafe.Pointer(cdir))
	C.GeoIP_setup_custom_directory(cdir)
}

// OpenType opens a specified GeoIP database type in the default location with the
// specified GeoIPOptions flag. Constants are defined for each database type
// (for example GEOIP_COUNTRY_EDITION).
func OpenTypeFlag(dbType int, flag int) (*GeoIP, error) {
	g := &GeoIP{}
	runtime.SetFinalizer(g, (*GeoIP).free)

	var err error

	g.db, err = C.GeoIP_open_type(C.int(dbType), C.int(flag))
	if err != nil {
		return nil, fmt.Errorf("Error opening GeoIP database (%d): %s", dbType, err)
	}

	if g.db == nil {
		return nil, fmt.Errorf("Didn't open GeoIP database (%d)", dbType)
	}

	C.GeoIP_set_charset(g.db, C.GEOIP_CHARSET_UTF8)

	return g, nil
}

// OpenType opens a specified GeoIP database type in the default location
// and the 'memory cache' flag. Use OpenTypeFlag() to specify flag.
func OpenType(dbType int) (*GeoIP, error) {
	return OpenTypeFlag(dbType, GEOIP_MEMORY_CACHE)
}

// Takes an IPv4 address string and returns the organization name for that IP.
// Requires the GeoIP organization database.
func (gi *GeoIP) GetOrg(ip string) string {
	name, _ := gi.GetName(ip)
	return name
}

// Works on the ASN, Netspeed, Organization and probably other
// databases, takes and IP string and returns a "name" and the
// netmask.
func (gi *GeoIP) GetName(ip string) (name string, netmask int) {
	if gi.db == nil {
		return
	}

	gi.mu.Lock()
	defer gi.mu.Unlock()

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))
	cname := C.GeoIP_name_by_addr(gi.db, cip)

	if cname != nil {
		name = C.GoString(cname)
		defer C.free(unsafe.Pointer(cname))
		netmask = int(C.GeoIP_last_netmask(gi.db))
		return
	}
	return
}

type GeoIPRecord struct {
	CountryCode   string
	CountryCode3  string
	CountryName   string
	Region        string
	City          string
	PostalCode    string
	Latitude      float32
	Longitude     float32
	MetroCode     int
	AreaCode      int
	CharSet       int
	ContinentCode string
}

// Returns the "City Record" for an IP address. Requires the GeoCity(Lite)
// database - http://www.maxmind.com/en/city
func (gi *GeoIP) GetRecord(ip string) *GeoIPRecord {
	if gi.db == nil {
		return nil
	}

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))

	gi.mu.Lock()
	record := C.GeoIP_record_by_addr(gi.db, cip)
	gi.mu.Unlock()

	if record == nil {
		return nil
	}
	// defer C.free(unsafe.Pointer(record))
	defer C.GeoIPRecord_delete(record)
	rec := new(GeoIPRecord)
	rec.CountryCode = C.GoString(record.country_code)
	rec.CountryCode3 = C.GoString(record.country_code3)
	rec.CountryName = C.GoString(record.country_name)
	rec.Region = C.GoString(record.region)
	rec.City = C.GoString(record.city)
	rec.PostalCode = C.GoString(record.postal_code)
	rec.Latitude = float32(record.latitude)
	rec.Longitude = float32(record.longitude)
	rec.CharSet = int(record.charset)
	rec.ContinentCode = C.GoString(record.continent_code)

	if gi.db.databaseType != C.GEOIP_CITY_EDITION_REV0 {
		/* DIRTY HACK BELOW:
		   The GeoIPRecord struct in GeoIPCity.h contains an int32 union of metro_code and dma_code.
		   The union is unnamed, so cgo names it anon0 and assumes it's a 4-byte array.
		*/
		union_int := (*int32)(unsafe.Pointer(&record.anon0))
		rec.MetroCode = int(*union_int)
		rec.AreaCode = int(record.area_code)
	}

	return rec
}

// Returns the country code and region code for an IP address. Requires
// the GeoIP Region database.
func (gi *GeoIP) GetRegion(ip string) (string, string) {
	if gi.db == nil {
		return "", ""
	}

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))

	gi.mu.Lock()
	region := C.GeoIP_region_by_addr(gi.db, cip)
	gi.mu.Unlock()

	if region == nil {
		return "", ""
	}

	countryCode := C.GoString(&region.country_code[0])
	regionCode := C.GoString(&region.region[0])
	defer C.free(unsafe.Pointer(region))

	return countryCode, regionCode
}

// Returns the region name given a country code and region code
func GetRegionName(countryCode, regionCode string) string {

	cc := C.CString(countryCode)
	defer C.free(unsafe.Pointer(cc))

	rc := C.CString(regionCode)
	defer C.free(unsafe.Pointer(rc))

	region := C.GeoIP_region_name_by_code(cc, rc)
	if region == nil {
		return ""
	}

	// it's a static string constant, don't free this
	regionName := C.GoString(region)

	return regionName
}

// Same as GetName() but for IPv6 addresses.
func (gi *GeoIP) GetNameV6(ip string) (name string, netmask int) {
	if gi.db == nil {
		return
	}

	gi.mu.Lock()
	defer gi.mu.Unlock()

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))
	cname := C.GeoIP_name_by_addr_v6(gi.db, cip)

	if cname != nil {
		name = C.GoString(cname)
		defer C.free(unsafe.Pointer(cname))
		netmask = int(C.GeoIP_last_netmask(gi.db))
		return
	}
	return
}

// Takes an IPv4 address string and returns the country code for that IP
// and the netmask for that IP range.
func (gi *GeoIP) GetCountry(ip string) (cc string, netmask int) {
	if gi.db == nil {
		return
	}

	gi.mu.Lock()
	defer gi.mu.Unlock()

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))
	ccountry := C.GeoIP_country_code_by_addr(gi.db, cip)

	if ccountry != nil {
		cc = C.GoString(ccountry)
		netmask = int(C.GeoIP_last_netmask(gi.db))
		return
	}
	return
}

// GetCountry_v6 works the same as GetCountry except for IPv6 addresses, be sure to
// load a database with IPv6 data to get any results.
func (gi *GeoIP) GetCountry_v6(ip string) (cc string, netmask int) {
	if gi.db == nil {
		return
	}

	gi.mu.Lock()
	defer gi.mu.Unlock()

	cip := C.CString(ip)
	defer C.free(unsafe.Pointer(cip))
	ccountry := C.GeoIP_country_code_by_addr_v6(gi.db, cip)
	if ccountry != nil {
		cc = C.GoString(ccountry)
		netmask = int(C.GeoIP_last_netmask(gi.db))
		return
	}
	return
}
