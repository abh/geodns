package querylog

import (
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"
)

func TestAvro(t *testing.T) {

	tmppath, err := os.MkdirTemp("", "geodns.avro")
	if err != nil {
		t.Fatalf("could not create temp dir: %s", err)
	}

	lg, err := NewAvroLogger(tmppath, 5000000, 4*time.Second)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	dataFh, err := os.Open("testdata/queries.log")
	if err != nil {
		t.Log("no test data available")
		t.SkipNow()
	}
	dec := json.NewDecoder(dataFh)

	count := 0
	for {
		e := Entry{}
		err := dec.Decode(&e)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Logf("could not decode test data: %s", err)
			continue
		}
		count++
		lg.Write(&e)
	}

	t.Logf("Write count: %d", count)

	// time.Sleep(time.Second * 2)

	err = lg.Close()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}
