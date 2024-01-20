package modules_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reporter"
	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories"
)

func makeApp(extra ...fx.Option) (*fx.App, func()) {
	fxopts := []fx.Option{
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ReporterListenAddr: "127.0.0.1:33811",
				ReporterBufferSize: 1024,
			}
		}),
		reporter.Module,
		fx.NopLogger,
		fx.Invoke(func(*reporter.Reporter) {}),
	}
	fxopts = append(fxopts, extra...)
	app := fx.New(fxopts...)
	return app, func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}
}

func TestReporter_Available_OK(t *testing.T) {
	ctx := context.TODO()
	app, cancel := makeApp()
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	resp := testutils.SendUDP("127.0.0.1:33811", []byte{0x09}) // nolint: errcheck
	assert.Equal(t, []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}, resp)
}

func TestReporter_Challenge_OK(t *testing.T) {
	ctx := context.TODO()

	app, cancel := makeApp()
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	resp := testutils.SendUDP("127.0.0.1:33811", []byte{0x01, 0xfa, 0xca, 0xde, 0xaf}) // nolint: errcheck
	assert.Equal(t, []byte{0xfe, 0xfd, 0x0a, 0xfa, 0xca, 0xde, 0xaf}, resp)
}

func TestReporter_Challenge_InvalidInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantResp []byte
		wantErr  bool
	}{
		{
			name:     "positive case",
			payload:  []byte{0x01, 0xfe, 0xed, 0xf0, 0x0d},
			wantResp: []byte{0xfe, 0xfd, 0x0a, 0xfe, 0xed, 0xf0, 0x0d},
		},
		{
			name:     "positive edge case - all nulls",
			payload:  []byte{0x01, 0x00, 0x00, 0x00, 0x00},
			wantResp: []byte{0xfe, 0xfd, 0x0a, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name:    "insufficient payload length #1",
			payload: []byte{0x01},
			wantErr: true,
		},
		{
			name:    "insufficient payload length #2",
			payload: []byte{0x01, 0xfe, 0xed},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			app, cancel := makeApp()
			defer cancel()
			app.Start(ctx) // nolint: errcheck

			client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)
			defer client.Close()
			resp, err := client.Send(tt.payload)

			if tt.wantErr {
				require.ErrorIs(t, err, os.ErrDeadlineExceeded)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestReporter_Heartbeat_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var instanceRepo repositories.InstanceRepository
	var probeRepo repositories.ProbeRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo, &instanceRepo, &probeRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	req := testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams())
	client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Duration(0))
	defer client.Close()
	resp, err := client.Send(req)
	require.NoError(t, err)

	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	assert.Equal(t, resp[3:7], []byte{0xfe, 0xed, 0xf0, 0x0d})
	assert.Equal(t, resp[7:13], []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59})

	respAddr := make([]byte, 7)
	hex.Decode(respAddr, resp[13:27]) // nolint: errcheck
	assert.Equal(t, respAddr[0], uint8(0x00))
	assert.Equal(t, "127.0.0.1", net.IPv4(respAddr[1], respAddr[2], respAddr[3], respAddr[4]).String())
	assert.Equal(t, client.LocalAddr.Port, int(binary.BigEndian.Uint16(respAddr[5:7])))
	assert.Equal(t, uint8(0x00), resp[27])
}

func TestReporter_Heartbeat_ServerIsAddedAndThenUpdated(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var instanceRepo repositories.InstanceRepository
	var probeRepo repositories.ProbeRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo, &instanceRepo, &probeRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
		"localport":  "10484",
	})
	req := testutils.PackHeartbeatRequest(instanceID, paramsBefore)
	testutils.SendUDP("127.0.0.1:33811", req)

	// server is stored with the correct discovery status
	svr, err := serverRepo.Get(ctx, addr.MustNewFromDotted("127.0.0.1", 10480))
	assert.NoError(t, err)
	assert.Equal(t, 10480, svr.Addr.Port)
	assert.Equal(t, 10480, svr.Info.HostPort)
	assert.Equal(t, 10484, svr.QueryPort)
	assert.Equal(t, ds.Master|ds.Info|ds.PortRetry, svr.DiscoveryStatus)
	assert.Equal(t, "Swat4 Server", svr.Info.Hostname)
	assert.Equal(t, "VIP Escort", svr.Info.GameType)
	assert.Equal(t, "A-Bomb Nightclub", svr.Info.MapName)

	// instance is stored with the server's address
	inst, err := instanceRepo.GetByID(ctx, string([]byte{0xfe, 0xed, 0xf0, 0x0d}))
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:10480", inst.Addr.String())

	// probe is added to discover the server's port
	prb, err := probeRepo.PopAny(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:10480", prb.Addr.String())
	assert.Equal(t, 10480, prb.Port)
	assert.Equal(t, probe.GoalPort, prb.Goal)

	paramsAfter := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	req = testutils.PackHeartbeatRequest(instanceID, paramsAfter)
	testutils.SendUDP("127.0.0.1:33811", req)

	// server is updated with the new info
	svr, err = serverRepo.Get(ctx, addr.MustNewFromDotted("127.0.0.1", 10480))
	assert.NoError(t, err)
	assert.Equal(t, 15, svr.Info.NumPlayers)
	assert.Equal(t, "VIP Escort", svr.Info.GameType)
	assert.Equal(t, "Food Wall Restaurant", svr.Info.MapName)
}

func TestReporter_Heartbeat_ServerIsUpdatedWithNewInstanceID(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var instanceRepo repositories.InstanceRepository
	var probeRepo repositories.ProbeRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo, &instanceRepo, &probeRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	oldInstanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
		"localport":  "10484",
	})
	req := testutils.PackHeartbeatRequest(oldInstanceID, paramsBefore)
	testutils.SendUDP("127.0.0.1:33811", req)

	instance, err := instanceRepo.GetByID(ctx, string(oldInstanceID))
	require.NoError(t, err)
	svr, err := serverRepo.Get(ctx, instance.Addr)
	require.NoError(t, err)
	assert.Equal(t, 16, svr.Info.NumPlayers)
	assert.Equal(t, "VIP Escort", svr.Info.GameType)
	assert.Equal(t, "A-Bomb Nightclub", svr.Info.MapName)
	assert.Equal(t, "127.0.0.1", svr.Addr.GetDottedIP())

	newParams := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "Barricaded Suspects",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	newInstanceID := []byte{0xde, 0xad, 0xbe, 0xef}
	req = testutils.PackHeartbeatRequest(newInstanceID, newParams)
	testutils.SendUDP("127.0.0.1:33811", req)

	// server is updated with the new info
	svr, err = serverRepo.Get(ctx, addr.MustNewFromDotted("127.0.0.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, 15, svr.Info.NumPlayers)
	assert.Equal(t, "Barricaded Suspects", svr.Info.GameType)
	assert.Equal(t, "Food Wall Restaurant", svr.Info.MapName)
	assert.Equal(t, "127.0.0.1", svr.Addr.GetDottedIP())

	// at the same time, the server is no longer accessible by the former instance key
	_, getErr := instanceRepo.GetByID(ctx, string(oldInstanceID))
	assert.ErrorIs(t, getErr, repositories.ErrInstanceNotFound)
}

func TestReporter_Heartbeat_ServerPortIsDiscovered(t *testing.T) {
	tests := []struct {
		name           string
		isNew          bool
		initStatus     ds.DiscoveryStatus
		wantDiscovered bool
		wantStatus     ds.DiscoveryStatus
	}{
		{
			"new server is queued",
			true,
			ds.NoStatus,
			true,
			ds.Info | ds.Master | ds.PortRetry,
		},
		{
			"existing server is queued",
			false,
			ds.Info | ds.Details,
			true,
			ds.Master | ds.Info | ds.Details | ds.PortRetry,
		},
		{
			"existing server is ignored - port already queued",
			false,
			ds.Info | ds.Details | ds.PortRetry,
			false,
			ds.Master | ds.Info | ds.Details | ds.PortRetry,
		},
		{
			"existing server is ignored - port already set",
			false,
			ds.Master | ds.Port,
			false,
			ds.Master | ds.Info | ds.Port,
		},
		{
			"existing server is ignored - port retried",
			false,
			ds.Port | ds.PortRetry,
			false,
			ds.Master | ds.Info | ds.Port | ds.PortRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverRepo repositories.ServerRepository
			var instanceRepo repositories.InstanceRepository
			var probeRepo repositories.ProbeRepository

			ctx := context.TODO()
			app, cancel := makeApp(
				fx.Populate(&serverRepo, &instanceRepo, &probeRepo),
			)
			defer cancel()
			app.Start(ctx) // nolint: errcheck

			if !tt.isNew {
				svr := server.MustNew(net.ParseIP("127.0.0.1"), 10480, 10484)
				if tt.initStatus.HasStatus() {
					svr.UpdateDiscoveryStatus(tt.initStatus)
				}
				serverRepo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck
			}

			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			params := testutils.GenExtraServerParams(map[string]string{
				"gametype":   "VIP Escort",
				"mapname":    "A-Bomb Nightclub",
				"numplayers": "16",
				"hostport":   "10480",
				"localport":  "10484",
			})
			req := testutils.PackHeartbeatRequest(instanceID, params)
			testutils.SendUDP("127.0.0.1:33811", req)

			reportedSvr, err := serverRepo.Get(ctx, addr.MustNewFromDotted("127.0.0.1", 10480))
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, reportedSvr.DiscoveryStatus)

			probeCount, err := probeRepo.Count(ctx)
			require.NoError(t, err)

			if tt.wantDiscovered {
				assert.Equal(t, 1, probeCount)
				prb, err := probeRepo.Pop(ctx)
				require.NoError(t, err)
				assert.Equal(t, probe.GoalPort, prb.Goal)
				assert.Equal(t, "127.0.0.1:10480", prb.Addr.String())
				assert.Equal(t, 10480, prb.Port)
			} else {
				assert.Equal(t, 0, probeCount)
			}
		})
	}
}

func TestReporter_Heartbeat_ServerIsRefreshed(t *testing.T) {
	var serverRepo repositories.ServerRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	req := testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams())

	// initial report
	testutils.SendUDP("127.0.0.1:33811", req)

	afterInitial := time.Now()

	updatedAfterInitial, _ := serverRepo.Filter(
		ctx,
		filterset.New().ActiveAfter(afterInitial).WithStatus(ds.Master),
	)
	assert.Len(t, updatedAfterInitial, 0)

	// successive report refreshes the server
	testutils.SendUDP("127.0.0.1:33811", req)

	updatedAfterInitialRepeated, _ := serverRepo.Filter(
		ctx,
		filterset.New().ActiveAfter(afterInitial).WithStatus(ds.Master),
	)
	assert.Len(t, updatedAfterInitialRepeated, 1)
}

func TestReporter_Heartbeat_ServerIsRemoved(t *testing.T) {
	var serverRepo repositories.ServerRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)

	reportReq := testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams())
	resp, err := client.Send(reportReq)
	require.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})

	serverCount, _ := serverRepo.Count(ctx)
	assert.Equal(t, 1, serverCount)

	removeReq := testutils.PackHeartbeatRequest(
		[]byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
	)

	// remove the server by sending param statechanged=2
	_, err = client.Send(removeReq)
	// expect no response
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	serverCount, _ = serverRepo.Count(ctx)
	assert.Equal(t, 0, serverCount)

	// subsequent statechanged=2 requests should produce no errors
	_, err = client.Send(removeReq)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)

	serverCount, _ = serverRepo.Count(ctx)
	assert.Equal(t, 0, serverCount)
}

func TestReporter_Heartbeat_ServerRemovalIsValidated(t *testing.T) {
	tests := []struct {
		name        string
		instanceID  []byte
		params      map[string]string
		ipaddr      string
		wantSuccess bool
	}{
		{
			name:        "positive case",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "127.0.0.1",
			wantSuccess: true,
		},
		{
			name:        "statechanged != 2",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "1"}),
			ipaddr:      "127.0.0.1",
			wantSuccess: false, // no error but the request is processed as a normal heartbeat message
		},
		{
			name:        "unknown server",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "127.0.0.2",
			wantSuccess: false, // no error, could be a subsequent removal request of a former server
		},
		{
			name:        "unknown instance id",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "127.0.0.1",
			wantSuccess: false,
		},
		{
			name:        "ip addr does not match",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "2.2.2.2",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverRepo repositories.ServerRepository
			var instanceRepo repositories.InstanceRepository
			var metrics *monitoring.MetricService

			ctx := context.TODO()
			app, cancel := makeApp(
				fx.Populate(&serverRepo, &instanceRepo, &metrics),
			)
			defer cancel()
			app.Start(ctx) // nolint: errcheck

			client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)

			svr := factories.BuildServer(factories.WithAddress(tt.ipaddr, 10480), factories.WithQueryPort(10484))
			inst := instance.MustNew(string([]byte{0xfe, 0xed, 0xf0, 0x0d}), svr.Addr.GetIP(), svr.Addr.Port)
			serverRepo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck
			instanceRepo.Add(ctx, inst)                                   // nolint: errcheck

			// removal request
			removeReq := testutils.PackHeartbeatRequest(tt.instanceID, tt.params)
			client.Send(removeReq) // nolint: errcheck

			serverCount, _ := serverRepo.Count(ctx)
			metricValue := testutil.ToFloat64(metrics.ReporterRemovals)
			if tt.wantSuccess {
				assert.Equal(t, 0, serverCount)
				assert.Equal(t, float64(1), metricValue)
			} else {
				assert.Equal(t, 1, serverCount)
				assert.Equal(t, float64(0), metricValue)
			}
		})
	}
}

func TestReporter_Heartbeat_InvalidPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantErr bool
	}{
		{
			name: "positive case",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				testutils.GenServerParams(),
			),
			wantErr: false,
		},
		{
			name:    "positive edge case - null instance id",
			payload: testutils.PackHeartbeatRequest([]byte{0xfe, 0x00, 0x00, 0x0d}, testutils.GenServerParams()),
			wantErr: false,
		},
		{
			name:    "insufficient payload length #1",
			payload: []byte{0x03},
			wantErr: true,
		},
		{
			name:    "insufficient payload length #2",
			payload: []byte{0x03, 0xfe, 0xed, 0xf0, 0x0d},
			wantErr: true,
		},
		{
			name:    "no fields are present",
			payload: testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, nil),
			wantErr: true,
		},
		{
			name: "no known fields are present",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				map[string]string{"somefield": "Swat4 Server", "other": "1.1"},
			),
			wantErr: true,
		},
		{
			name: "field has no value",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				map[string]string{"hostname": "Swat4 Server", "gamever": ""},
			),
			wantErr: true,
		},
		{
			name: "invalid field value",
			payload: testutils.PackHeartbeatRequest(
				[]byte{0xfe, 0xed, 0xf0, 0x0d},
				testutils.GenExtraServerParams(map[string]string{"numplayers": "-1"}),
			),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			app, cancel := makeApp()
			defer cancel()
			app.Start(ctx) // nolint: errcheck

			client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)
			defer client.Close()
			resp, err := client.Send(tt.payload)

			if tt.wantErr {
				require.ErrorIs(t, err, os.ErrDeadlineExceeded)
			} else {
				require.NoError(t, err)
				assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
			}
		})
	}
}

func TestReporter_Keepalive_ServerIsRefreshed(t *testing.T) {
	var serverRepo repositories.ServerRepository

	ctx := context.TODO()
	app, cancel := makeApp(
		fx.Populate(&serverRepo),
	)
	defer cancel()
	app.Start(ctx) // nolint: errcheck

	client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)

	// remember the time before the server is reported
	beforeInitial := time.Now()

	// initial report
	reportReq := testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams())
	_, err := client.Send(reportReq)
	require.NoError(t, err)

	updatedAfterInitial, _ := serverRepo.Filter(
		ctx,
		filterset.New().ActiveAfter(beforeInitial).WithStatus(ds.Master),
	)
	assert.Len(t, updatedAfterInitial, 1)

	afterInitial := time.Now()
	updatedBeforeInitial, _ := serverRepo.Filter(
		ctx,
		filterset.New().ActiveAfter(afterInitial).WithStatus(ds.Master),
	)
	assert.Len(t, updatedBeforeInitial, 0)

	// keepalive request
	_, err = client.Send([]byte{0x08, 0xfe, 0xed, 0xf0, 0x0d})
	// no response expected
	assert.ErrorIs(t, err, os.ErrDeadlineExceeded)

	// server is refreshed
	updatedBeforeInitialRepeated, _ := serverRepo.Filter(
		ctx,
		filterset.New().ActiveAfter(afterInitial).WithStatus(ds.Master),
	)
	assert.Len(t, updatedBeforeInitialRepeated, 1)
}

func TestReporter_Keepalive_Errors(t *testing.T) {
	tests := []struct {
		name       string
		svrAddr    string
		instanceID []byte
		payload    []byte
		wantErr    bool
	}{
		{
			name:       "positive case",
			svrAddr:    "127.0.0.1",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			wantErr:    false,
		},
		{
			name:       "positive edge case - some nulls in instance id",
			svrAddr:    "127.0.0.1",
			instanceID: []byte{0xfe, 0x00, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0x00, 0xf0, 0x0d},
			wantErr:    false,
		},
		{
			name:       "unmatched ip address",
			svrAddr:    "1.0.0.2",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			wantErr:    true,
		},
		{
			name:       "unknown instance key",
			svrAddr:    "127.0.0.1",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xde, 0xad, 0xbe, 0xef},
			wantErr:    true,
		},
		{
			name:       "unacceptable payload - length",
			svrAddr:    "127.0.0.1",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var serverRepo repositories.ServerRepository
			var instanceRepo repositories.InstanceRepository

			ctx := context.TODO()
			app, cancel := makeApp(
				fx.Populate(&serverRepo, &instanceRepo),
			)
			defer cancel()
			app.Start(ctx) // nolint: errcheck

			client := testutils.NewUDPClient("127.0.0.1:33811", 1024, time.Millisecond*10)

			svr := server.MustNew(net.ParseIP(tt.svrAddr), 10480, 10484)
			inst := instance.MustNew(string(tt.instanceID), svr.Addr.GetIP(), svr.Addr.Port)
			serverRepo.Add(ctx, svr, repositories.ServerOnConflictIgnore) // nolint: errcheck
			instanceRepo.Add(ctx, inst)                                   // nolint: errcheck

			beforeKA := time.Now()

			// send keepalive request in a while
			_, err := client.Send(tt.payload)
			// no response expected
			assert.ErrorIs(t, err, os.ErrDeadlineExceeded)

			updatedAfterKA, _ := serverRepo.Filter(ctx, filterset.New().ActiveAfter(beforeKA))
			if tt.wantErr {
				assert.Len(t, updatedAfterKA, 0)
			} else {
				assert.Len(t, updatedAfterKA, 1)
			}
		})
	}
}
