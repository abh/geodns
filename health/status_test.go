package health

import "testing"

func TestStatusFile(t *testing.T) {
	sf := NewStatusFile()
	err := sf.Load("test.json")
	if err != nil {
		t.Fatalf("could not load test.json: %s", err)
	}
	x := sf.GetStatus("bad")

	t.Logf("bad=%d", x)

	if x != StatusUnhealthy {
		t.Errorf("'bad' should have been unhealthy but was %s", x.String())
	}
	registry.Add("test", sf)
}
