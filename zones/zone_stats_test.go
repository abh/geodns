package zones

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestZoneStats(t *testing.T) {
	zs := NewZoneLabelStats(4)
	if zs == nil {
		t.Fatalf("NewZoneLabelStats returned nil")
	}
	t.Log("adding 4 entries")
	zs.Add("abc")
	zs.Add("foo")
	zs.Add("def")
	zs.Add("abc")
	t.Log("getting counts")
	co := zs.Counts()
	assert.Equal(t, co["abc"], 2)
	assert.Equal(t, co["foo"], 1)

	zs.Add("foo")

	co = zs.Counts()
	assert.Equal(t, co["abc"], 1) // the first abc rolled off
	assert.Equal(t, co["foo"], 2)

	zs.Close()

	zs = NewZoneLabelStats(10)
	zs.Add("a")
	zs.Add("a")
	zs.Add("a")
	zs.Add("b")
	zs.Add("c")
	zs.Add("c")
	zs.Add("d")
	zs.Add("d")
	zs.Add("e")
	zs.Add("f")

	top := zs.TopCounts(2)
	assert.Len(t, top, 3, "TopCounts(2) returned 3 elements")
	assert.Equal(t, top[0].Label, "a")

	zs.Reset()
	assert.Len(t, zs.Counts(), 0, "Counts() is empty after reset")

	zs.Close()

}
