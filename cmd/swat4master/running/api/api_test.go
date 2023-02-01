package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	httpserver "github.com/sergeii/swat4master/cmd/swat4master/running/api"
)

func TestAPI_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	build.Commit = "foobar"
	build.Version = "v1.0.0"
	build.Time = "2022-04-24T11:22:33T"

	cfg := config.Config{
		HTTPListenAddr: "localhost:11337",
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(httpserver.Run, ctx)
	runner.WaitReady()

	// check status endpoint with build info
	resp, err := http.Get("http://localhost:11337/status")
	require.NoError(t, err)
	defer resp.Body.Close() // nolint: errcheck
	assert.Equal(t, 200, resp.StatusCode)
	statusInfo := make(map[string]string)
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &statusInfo) // nolint: errcheck
	assert.Equal(t, statusInfo, map[string]string{
		"BuildCommit":  "foobar",
		"BuildTime":    "2022-04-24T11:22:33T",
		"BuildVersion": "v1.0.0",
	})
	cancel()
	runner.WaitQuit()
}
