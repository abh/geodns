package maxminddb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworks(t *testing.T) {
	for _, recordSize := range []uint{24, 28, 32} {
		for _, ipVersion := range []uint{4, 6} {
			fileName := fmt.Sprintf("test-data/test-data/MaxMind-DB-test-ipv%d-%d.mmdb", ipVersion, recordSize)
			reader, err := Open(fileName)
			require.Nil(t, err, "unexpected error while opening database: %v", err)
			defer reader.Close()

			n := reader.Networks()
			for n.Next() {
				record := struct {
					IP string `maxminddb:"ip"`
				}{}
				network, err := n.Network(&record)
				assert.Nil(t, err)
				assert.Equal(t, record.IP, network.IP.String(),
					"expected %s got %s", record.IP, network.IP.String(),
				)
			}
			assert.Nil(t, n.Err())
		}
	}
}

func TestNetworksWithInvalidSearchTree(t *testing.T) {
	reader, err := Open("test-data/test-data/MaxMind-DB-test-broken-search-tree-24.mmdb")
	require.Nil(t, err, "unexpected error while opening database: %v", err)
	defer reader.Close()

	n := reader.Networks()
	for n.Next() {
		var record interface{}
		_, err := n.Network(&record)
		assert.Nil(t, err)
	}
	assert.NotNil(t, n.Err(), "no error received when traversing an broken search tree")
	assert.Equal(t, n.Err().Error(), "invalid search tree at 128.128.128.128/32")
}
