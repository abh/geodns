package monitor

import "testing"

func TestMonitor(t *testing.T) {

	// mux := dns.NewServeMux()
	// mm := zones.NewMuxManager("dns", mux)

	// // s.zones = make(zones.Zones)
	// metrics := NewMetrics()
	// go metrics.Updater()

	// *flaghttp = ":8881"

	// fmt.Println("Starting http server")

	// // TODO: use httptest
	// // https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/Jk785WB7F8I

	// srv := Server{}
	// srv.

	// todo: this isn't right, it should probably just take the mux?
	// go httpHandler(mm.Zones())
	// time.Sleep(500 * time.Millisecond)

	// c.Check(true, DeepEquals, true)

	// res, err := http.Get("http://localhost:8881/version")
	// c.Assert(err, IsNil)
	// page, _ := ioutil.ReadAll(res.Body)
	// c.Check(string(page), Matches, ".*<title>GeoDNS [0-9].*")

	// res, err = http.Get("http://localhost:8881/status")
	// c.Assert(err, IsNil)
	// page, _ = ioutil.ReadAll(res.Body)
	// // just check that template basically works

	// isOk := strings.Contains(string(page), "<html>")
	// // page has <html>
	// c.Check(isOk, Equals, true)

}
