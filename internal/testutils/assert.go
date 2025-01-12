package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/entities/server"
)

func AssertServers(t testing.TB, expected []string, actual []server.Server) {
	addresses := make([]string, 0, len(actual))
	for _, s := range actual {
		addresses = append(addresses, s.Addr.String())
	}
	assert.Equal(t, expected, addresses)
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func MustNoError(err error) {
	if err != nil {
		panic(err)
	}
}
