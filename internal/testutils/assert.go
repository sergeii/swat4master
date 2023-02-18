package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/servers"
)

func AssertServers(t testing.TB, expected []string, actual []servers.Server) {
	addresses := make([]string, 0, len(actual))
	for _, s := range actual {
		addresses = append(addresses, s.GetAddr().String())
	}
	assert.Equal(t, expected, addresses)
}
