package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/internal/testutils"
)

func TestAPI_Status_OK(t *testing.T) {
	var statusInfo map[string]string

	ts, cancel := testutils.PrepareTestServer(t)
	defer cancel()

	build.Commit = "foobar"
	build.Version = "v1.0.0"
	build.Time = "2022-04-24T11:22:33T"

	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/status", nil,
		testutils.MustBindJSON(&statusInfo),
	)

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, statusInfo, map[string]string{
		"BuildCommit":  "foobar",
		"BuildTime":    "2022-04-24T11:22:33T",
		"BuildVersion": "v1.0.0",
	})
}
