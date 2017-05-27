package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/abh/geodns/server"
	"github.com/abh/geodns/targeting"
	"github.com/abh/geodns/targeting/geoip2"
	"github.com/abh/geodns/zones"
)

func TestHTTP(t *testing.T) {

	geoprovider, err := geoip2.New("/usr/local/share/GeoIP")
	if err == nil {
		targeting.Setup(geoprovider)
	}

	// todo: less global metrics ...
	server.NewMetrics()

	mm, err := zones.NewMuxManager("dns", &zones.NilReg{})
	if err != nil {
		t.Fatalf("loading zones: %s", err)
	}
	hs := NewHTTPServer(mm, serverInfo)

	srv := httptest.NewServer(hs.Mux())

	baseurl := srv.URL
	t.Logf("server base url: '%s'", baseurl)

	// metrics := NewMetrics()
	// go metrics.Updater()

	res, err := http.Get(baseurl + "/version")
	require.Nil(t, err)
	page, _ := ioutil.ReadAll(res.Body)

	if !bytes.Contains(page, []byte("<title>GeoDNS")) {
		t.Log("/version didn't include '<title>GeoDNS'")
		t.Fail()
	}

	res, err = http.Get(baseurl + "/status")
	require.Nil(t, err)
	page, _ = ioutil.ReadAll(res.Body)

	// just check that template basically works
	if !bytes.Contains(page, []byte("<html>")) {
		t.Log("/status didn't include <html>")
		t.Fail()
	}
}
