package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/browser"
	httpserver "github.com/sergeii/swat4master/cmd/swat4master/subcommand/http"
	"github.com/sergeii/swat4master/cmd/swat4master/subcommand/reporter"
	"github.com/sergeii/swat4master/internal/api/monitoring"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/server/memory"
	"github.com/sergeii/swat4master/internal/testutils"
)

func TestRunHTTPServer_Smoke(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	build.Commit = "foobar"
	build.Version = "v1.0.0"
	build.Time = "2022-04-24T11:22:33T"

	cfg := config.Config{
		HTTPListenAddr: "localhost:11337",
	}
	gCtx := subcommand.NewGroupContext(&cfg, 1)
	go httpserver.Run(ctx, gCtx, application.NewApp())
	gCtx.WaitReady()

	// check status endpoint with build info
	resp, err := http.Get("http://localhost:11337/status")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	statusInfo := make(map[string]string)
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &statusInfo) // nolint: errcheck
	assert.Equal(t, statusInfo, map[string]string{
		"BuildCommit":  "foobar",
		"BuildTime":    "2022-04-24T11:22:33T",
		"BuildVersion": "v1.0.0",
	})

	// check metrics
	resp, err = http.Get("http://localhost:11337/metrics")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	parser := expfmt.TextParser{}
	mf, _ := parser.TextToMetricFamilies(resp.Body)
	assert.True(t, mf["go_goroutines"].Metric[0].Gauge.GetValue() > 0)

	cancel()
	gCtx.WaitQuit()
}

func TestRunHTTPServer_ReadServiceMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		HTTPListenAddr:       "localhost:11338",
		ReporterListenAddr:   "localhost:33811",
		ReporterBufferSize:   1024,
		BrowserListenAddr:    "localhost:13381",
		BrowserClientTimeout: time.Millisecond * 100,
	}
	gCtx := subcommand.NewGroupContext(&cfg, 3)
	app := application.NewApp(
		application.WithServerRepository(memory.New()),
		application.WithMetricService(monitoring.NewMetricService()),
	)
	go httpserver.Run(ctx, gCtx, app)
	go reporter.Run(ctx, gCtx, app)
	go browser.Run(ctx, gCtx, app)
	gCtx.WaitReady()

	buf := make([]byte, 2048)
	conn, _ := net.Dial("udp", "127.0.0.1:33811")
	// valid available request
	conn.Write([]byte{0x09}) // nolint: errcheck
	conn.Read(buf)           // nolint: errcheck

	// invalid keepalive request (no prior heartbeat)
	for i := 0; i < 2; i++ {
		conn.Write([]byte{0x08, 0xde, 0xad, 0xbe, 0xef}) // nolint: errcheck
	}

	// valid server browser request
	req := testutils.PackBrowserRequest(
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		"gametype='VIP Escort'",
		testutils.GenBrowserChallenge8,
		testutils.CalcReqLength,
	)
	conn, _ = net.Dial("tcp", "127.0.0.1:13381")
	_, err := conn.Write(req)
	require.NoError(t, err)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	conn.Close()

	// invalid browser request (no fields)
	req = testutils.PackBrowserRequest([]string{}, "", testutils.GenBrowserChallenge8, testutils.CalcReqLength)
	conn, _ = net.Dial("tcp", "127.0.0.1:13381")
	_, err = conn.Write(req)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 5)
	resp, err := http.Get("http://localhost:11338/metrics")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	parser := expfmt.TextParser{}
	mf, _ := parser.TextToMetricFamilies(resp.Body)

	assert.Equal(t, 11, int(mf["reporter_received_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 7, int(mf["reporter_sent_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["reporter_requests_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "available", *mf["reporter_requests_total"].Metric[0].Label[0].Value)
	assert.Equal(t, 2, int(mf["reporter_errors_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 180, int(mf["browser_received_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 133, int(mf["browser_sent_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["browser_requests_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["browser_errors_total"].Metric[0].Counter.GetValue()))

	cancel()
	gCtx.WaitQuit()
}
