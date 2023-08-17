package reporting_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	repos "github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/master/reporting"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	sp "github.com/sergeii/swat4master/internal/services/probe"
	ss "github.com/sergeii/swat4master/internal/services/server"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/validation"
)

func makeApp(tb fxtest.TB, extra ...fx.Option) {
	fxopts := []fx.Option{
		fx.Provide(clock.New),
		fx.Provide(func(c clock.Clock) (repos.ServerRepository, repos.InstanceRepository, repos.ProbeRepository) {
			mem := memory.New(c)
			return mem.Servers, mem.Instances, mem.Probes
		}),
		fx.Provide(validation.New),
		fx.Provide(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.Provide(
			monitoring.NewMetricService,
			ss.NewService,
			sp.NewService,
			finding.NewService,
			reporting.NewService,
		),
		fx.NopLogger,
	}
	fxopts = append(fxopts, extra...)
	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()
}

func overrideClock(c clock.Clock) fx.Option {
	return fx.Decorate(
		func() clock.Clock {
			return c
		},
	)
}

func TestReporter_DispatchAvailableRequest_OK(t *testing.T) {
	var service *reporting.Service
	makeApp(t, fx.Populate(&service))
	resp, msgT, err := service.DispatchRequest(context.TODO(), []byte{0x09}, &net.UDPAddr{})
	assert.NoError(t, err)
	assert.Equal(t, reporting.MasterMsgAvailable, msgT)
	assert.Equal(t, resp, []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00})
}

func TestReporter_DispatchChallengeRequest_OK(t *testing.T) {
	var service *reporting.Service
	makeApp(t, fx.Populate(&service))
	resp, msgT, err := service.DispatchRequest(context.TODO(), []byte{0x01, 0xfe, 0xed, 0xf0, 0x0d}, &net.UDPAddr{})
	assert.NoError(t, err)
	assert.Equal(t, reporting.MasterMsgChallenge, msgT)
	assert.Equal(t, resp, []byte{0xfe, 0xfd, 0x0a, 0xfe, 0xed, 0xf0, 0x0d})
}

func TestReporter_DispatchChallengeRequest_InvalidInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantResp []byte
		wantErr  error
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
			wantErr: reporting.ErrInvalidRequestPayload,
		},
		{
			name:    "insufficient payload length #2",
			payload: []byte{0x01, 0xfe, 0xed},
			wantErr: reporting.ErrInvalidRequestPayload,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *reporting.Service
			makeApp(t, fx.Populate(&service))
			resp, _, err := service.DispatchRequest(context.TODO(), tt.payload, &net.UDPAddr{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_OK(t *testing.T) {
	var service *reporting.Service
	makeApp(t, fx.Populate(&service))

	resp, msgT, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.NoError(t, err)
	assert.Equal(t, reporting.MasterMsgHeartbeat, msgT)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	assert.Equal(t, resp[3:7], []byte{0xfe, 0xed, 0xf0, 0x0d})
	assert.Equal(t, resp[7:13], []byte{0x44, 0x3d, 0x73, 0x7e, 0x6a, 0x59})

	respAddr := make([]byte, 7)
	hex.Decode(respAddr, resp[13:27]) // nolint: errcheck
	assert.Equal(t, respAddr[0], uint8(0x00))
	assert.Equal(t, "1.1.1.1", net.IPv4(respAddr[1], respAddr[2], respAddr[3], respAddr[4]).String())
	assert.Equal(t, 10481, int(binary.BigEndian.Uint16(respAddr[5:7])))
	assert.Equal(t, uint8(0x00), resp[27])
}

func TestReporter_DispatchHeartbeatRequest_ServerIsAddedAndUpdated(t *testing.T) {
	var service *reporting.Service
	var repo repos.ServerRepository

	ctx := context.TODO()
	makeApp(t, fx.Populate(&service, &repo))

	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10580",
		"localport":  "10584",
	})
	resp, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(instanceID, paramsBefore),
		&net.UDPAddr{IP: net.ParseIP("55.55.55.55"), Port: 22712}, // server is behind nat
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})

	svr, err := repo.Get(ctx, addr.MustNewFromString("55.55.55.55", 10580))
	require.NoError(t, err)

	details := svr.GetInfo()
	assert.Equal(t, 16, details.NumPlayers)
	assert.Equal(t, "VIP Escort", details.GameType)
	assert.Equal(t, "A-Bomb Nightclub", details.MapName)
	assert.Equal(t, "55.55.55.55", svr.GetDottedIP())
	assert.Equal(t, 10580, svr.GetGamePort())
	assert.Equal(t, 10584, svr.GetQueryPort())
	assert.Equal(t, ds.Master|ds.Info|ds.PortRetry, svr.GetDiscoveryStatus())

	paramsAfter := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	service.DispatchRequest( // nolint: errcheck
		context.TODO(),
		testutils.PackHeartbeatRequest(instanceID, paramsAfter),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	svr, _ = repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	details = svr.GetInfo()
	assert.Equal(t, 15, details.NumPlayers)
	assert.Equal(t, "VIP Escort", details.GameType)
	assert.Equal(t, "Food Wall Restaurant", details.MapName)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
}

func TestReporter_DispatchHeartbeatRequest_ServerIsUpdated(t *testing.T) {
	tests := []struct {
		name       string
		isNew      bool
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"new server is created",
			true,
			ds.NoStatus,
			ds.Info | ds.Master | ds.PortRetry,
		},
		{
			"existing server is updated",
			false,
			ds.Info | ds.Details,
			ds.Master | ds.Info | ds.Details | ds.PortRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *reporting.Service
			var repo repos.ServerRepository

			ctx := context.TODO()
			makeApp(t, fx.Populate(&service, &repo))

			if !tt.isNew {
				svr := server.MustNew(net.ParseIP("55.55.55.55"), 10580, 10584)
				if tt.initStatus.HasStatus() {
					svr.UpdateDiscoveryStatus(tt.initStatus)
				}
				repo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck
			}

			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			params := testutils.GenExtraServerParams(map[string]string{
				"gametype":   "VIP Escort",
				"mapname":    "A-Bomb Nightclub",
				"numplayers": "16",
				"hostport":   "10580",
				"localport":  "10581",
			})
			_, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest(instanceID, params),
				&net.UDPAddr{IP: net.ParseIP("55.55.55.55"), Port: 22712},
			)
			assert.NoError(t, err)

			reportedSvr, err := repo.Get(ctx, addr.MustNewFromString("55.55.55.55", 10580))
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, reportedSvr.GetDiscoveryStatus())
			assert.Equal(t, "55.55.55.55", reportedSvr.GetDottedIP())
			assert.Equal(t, 10580, reportedSvr.GetGamePort())

			if tt.isNew {
				assert.Equal(t, 10581, reportedSvr.GetQueryPort())
			} else {
				assert.Equal(t, 10584, reportedSvr.GetQueryPort())
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_ServerPortIsDiscovered(t *testing.T) {
	tests := []struct {
		name         string
		isNew        bool
		initStatus   ds.DiscoveryStatus
		isDiscovered bool
		wantStatus   ds.DiscoveryStatus
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
			var service *reporting.Service
			var serversRepo repos.ServerRepository
			var probesRepo repos.ProbeRepository

			ctx := context.TODO()
			makeApp(t, fx.Populate(&service, &serversRepo, &probesRepo))

			if !tt.isNew {
				svr := server.MustNew(net.ParseIP("55.55.55.55"), 10580, 10584)
				if tt.initStatus.HasStatus() {
					svr.UpdateDiscoveryStatus(tt.initStatus)
				}
				serversRepo.Add(ctx, svr, repos.ServerOnConflictIgnore) // nolint: errcheck
			}

			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			params := testutils.GenExtraServerParams(map[string]string{
				"gametype":   "VIP Escort",
				"mapname":    "A-Bomb Nightclub",
				"numplayers": "16",
				"hostport":   "10580",
				"localport":  "10584",
			})
			_, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest(instanceID, params),
				&net.UDPAddr{IP: net.ParseIP("55.55.55.55"), Port: 22712},
			)
			assert.NoError(t, err)

			reportedSvr, err := serversRepo.Get(ctx, addr.MustNewFromString("55.55.55.55", 10580))
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, reportedSvr.GetDiscoveryStatus())

			queueCount, err := probesRepo.Count(ctx)
			require.NoError(t, err)

			if tt.isDiscovered {
				assert.Equal(t, 1, queueCount)
				tgt, err := probesRepo.Pop(ctx)
				require.NoError(t, err)
				assert.Equal(t, probe.GoalPort, tgt.GetGoal())
				assert.Equal(t, "55.55.55.55:10580", tgt.GetAddr().String())
				assert.Equal(t, 10580, tgt.GetPort())
			} else {
				assert.Equal(t, 0, queueCount)
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_HandleServerBehindNAT(t *testing.T) {
	var service *reporting.Service
	var repo repos.ServerRepository

	ctx := context.TODO()
	clockMock := clock.NewMock()

	makeApp(t, fx.Populate(&service, &repo), overrideClock(clockMock))

	before := clockMock.Now()
	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	paramsBefore := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
		"localport":  "10484",
	})
	resp, err := testutils.SendHeartbeat(
		service, instanceID,
		testutils.WithServerParams(paramsBefore),
		// server is behind nat, connection port is different from the query port
		testutils.WithCustomAddr("1.1.1.1", 22712),
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	svr, _ := repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	details := svr.GetInfo()
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 16, details.NumPlayers)
	assert.Equal(t, 10480, details.HostPort)
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10484, svr.GetQueryPort())
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
	assert.False(t, svr.HasDiscoveryStatus(ds.Details))

	paramsAfter := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
		"localport":  "10484",
	})
	testutils.SendHeartbeat( // nolint: errcheck
		service, instanceID,
		testutils.WithServerParams(paramsAfter),
		testutils.WithCustomAddr("1.1.1.1", 37122),
	)
	svr, _ = repo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	details = svr.GetInfo()
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	assert.Equal(t, 15, details.NumPlayers)
	assert.Equal(t, 10480, details.HostPort)
	assert.Equal(t, 10480, svr.GetGamePort())
	assert.Equal(t, 10484, svr.GetQueryPort())

	svrs, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, svrs, 1)
}

func TestReporter_DispatchHeartbeatRequest_ServerIsUpdatedWithNewInstanceID(t *testing.T) {
	var service *reporting.Service
	var serversRepo repos.ServerRepository
	var instancesRepo repos.InstanceRepository

	ctx := context.TODO()
	makeApp(t, fx.Populate(&service, &serversRepo, &instancesRepo))

	oldInstanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	params := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
	})
	resp, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest(oldInstanceID, params),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})

	instance, err := instancesRepo.GetByID(ctx, string(oldInstanceID))
	require.NoError(t, err)
	svr, _ := serversRepo.Get(ctx, instance.GetAddr())
	details := svr.GetInfo()
	assert.Equal(t, 16, details.NumPlayers)
	assert.Equal(t, "VIP Escort", details.GameType)
	assert.Equal(t, "A-Bomb Nightclub", details.MapName)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())

	newParams := testutils.GenExtraServerParams(map[string]string{
		"gametype":   "Barricaded Suspects",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	newInstanceID := []byte{0xde, 0xad, 0xbe, 0xef}
	service.DispatchRequest( // nolint: errcheck
		context.TODO(),
		testutils.PackHeartbeatRequest(newInstanceID, newParams),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	svr, _ = serversRepo.Get(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	details = svr.GetInfo()
	assert.Equal(t, 15, details.NumPlayers)
	assert.Equal(t, "Barricaded Suspects", details.GameType)
	assert.Equal(t, "Food Wall Restaurant", details.MapName)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())

	// at the same time the server is no longer accessible by the former instance key
	_, getErr := instancesRepo.GetByID(ctx, string(oldInstanceID))
	assert.ErrorIs(t, getErr, repos.ErrInstanceNotFound)
}

func TestReporter_DispatchHeartbeatRequest_InvalidPayload(t *testing.T) {
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
			var service *reporting.Service
			makeApp(t, fx.Populate(&service))
			resp, _, err := service.DispatchRequest(
				context.TODO(),
				tt.payload,
				&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
			)
			if tt.wantErr {
				require.ErrorIs(t, err, reporting.ErrInvalidRequestPayload)
			} else {
				require.NoError(t, err)
				assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_OnlyIPv4IsSupported(t *testing.T) {
	tests := []struct {
		name    string
		ipaddr  string
		wantErr error
	}{
		{
			name:    "positive case",
			ipaddr:  "1.1.1.1",
			wantErr: nil,
		},
		{
			name:    "IPv6 is not supported",
			ipaddr:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantErr: addr.ErrInvalidServerIP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *reporting.Service
			makeApp(t, fx.Populate(&service))
			resp, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
			}
		})
	}
}

func TestReporter_DispatchHeartbeatRequest_ServerLivenessIsRefreshed(t *testing.T) {
	var service *reporting.Service
	var repo repos.ServerRepository

	ctx := context.TODO()
	clockMock := clock.NewMock()

	makeApp(t, fx.Populate(&service, &repo), overrideClock(clockMock))

	// initial report
	resp, err := testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})

	clockMock.Add(time.Millisecond)

	before := clockMock.Now()
	reportedSinceBefore, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 0)

	// successive report refreshes the server
	resp, err = testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	reportedSinceBefore, _ = repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 1)
}

func TestReporter_DispatchHeartbeatRequest_ServerIsRemoved(t *testing.T) {
	var service *reporting.Service
	var repo repos.ServerRepository

	ctx := context.TODO()
	clockMock := clock.NewMock()

	makeApp(t, fx.Populate(&service, &repo), overrideClock(clockMock))

	before := clockMock.Now()
	resp, err := testutils.SendHeartbeat(
		service, []byte{0xfe, 0xed, 0xf0, 0x0d},
		testutils.GenServerParams, testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Equal(t, resp[:3], []byte{0xfe, 0xfd, 0x01})
	reportedSinceBefore, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 1)

	// remove the server by sending param statechanged=2
	resp, err = testutils.SendHeartbeat(
		service,
		[]byte{0xfe, 0xed, 0xf0, 0x0d},
		func() map[string]string {
			return testutils.GenExtraServerParams(map[string]string{"statechanged": "2"})
		},
		testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Empty(t, resp) // no response
	reportedSinceBefore, _ = repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 0)

	// subsequent statechanged=2 requests should produce no errors
	resp, err = testutils.SendHeartbeat(
		service,
		[]byte{0xfe, 0xed, 0xf0, 0x0d},
		func() map[string]string {
			return testutils.GenExtraServerParams(map[string]string{"statechanged": "2"})
		},
		testutils.StandardAddr,
	)
	assert.NoError(t, err)
	assert.Empty(t, resp)
	reportedSinceBefore, _ = repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 0)

	remainingServers, _ := repo.Count(ctx)
	assert.Equal(t, 0, remainingServers)
}

func TestReporter_DispatchHeartbeatRequest_ServerRemovalIsValidated(t *testing.T) {
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
			ipaddr:      "1.1.1.1",
			wantSuccess: true,
		},
		{
			name:        "statechanged != 2",
			instanceID:  []byte{0xfe, 0xed, 0xf0, 0x0d},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "1"}),
			ipaddr:      "1.1.1.1",
			wantSuccess: false, // no error but the request is processed as a normal heartbeat message
		},
		{
			name:        "unknown server",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "2.2.2.2",
			wantSuccess: false, // no error, could be a subsequent removal request of a former server
		},
		{
			name:        "invalid instance id",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "1.1.1.1",
			wantSuccess: false,
		},
		{
			name:        "ip addr does not match",
			instanceID:  []byte{0xde, 0xad, 0xbe, 0xef},
			params:      testutils.GenExtraServerParams(map[string]string{"statechanged": "2"}),
			ipaddr:      "2.2.2.2",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *reporting.Service
			var metrics *monitoring.MetricService
			var repo repos.ServerRepository

			ctx := context.TODO()
			clockMock := clock.NewMock()

			makeApp(t, fx.Populate(&service, &metrics, &repo), overrideClock(clockMock))

			before := clockMock.Now()

			// initial report
			_, _, err := service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
			)
			require.NoError(t, err)

			// removal request
			_, _, err = service.DispatchRequest(
				context.TODO(),
				testutils.PackHeartbeatRequest(tt.instanceID, tt.params),
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			require.NoError(t, err)

			fs := repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master)
			reportedSinceBefore, _ := repo.Filter(ctx, fs)
			metricValue := testutil.ToFloat64(metrics.ReporterRemovals)
			if tt.wantSuccess {
				assert.Len(t, reportedSinceBefore, 0)
				assert.Equal(t, float64(1), metricValue)
			} else {
				assert.Len(t, reportedSinceBefore, 1)
				assert.Equal(t, float64(0), metricValue)
			}
		})
	}
}

func TestReporter_DispatchKeepaliveRequest_RefreshesServerLiveness(t *testing.T) {
	var service *reporting.Service
	var repo repos.ServerRepository

	ctx := context.TODO()
	clockMock := clock.NewMock()
	makeApp(t, fx.Populate(&service, &repo), overrideClock(clockMock))

	before := clockMock.Now()
	// initial report
	_, _, err := service.DispatchRequest(
		context.TODO(),
		testutils.PackHeartbeatRequest([]byte{0xfe, 0xed, 0xf0, 0x0d}, testutils.GenServerParams()),
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	require.NoError(t, err)
	reportedSinceBefore, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(before).WithStatus(ds.Master))
	assert.Len(t, reportedSinceBefore, 1)

	clockMock.Add(time.Millisecond)

	after := clockMock.Now()
	reportedSinceAfter, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(after).WithStatus(ds.Master))
	assert.Len(t, reportedSinceAfter, 0)

	resp, _, _ := service.DispatchRequest(
		context.TODO(),
		[]byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
		&net.UDPAddr{IP: net.ParseIP("1.1.1.1"), Port: 10481},
	)
	assert.Empty(t, resp)
	// the server is now live again
	reportedSinceAfter, _ = repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(after).WithStatus(ds.Master))
	assert.Len(t, reportedSinceAfter, 1)
}

func TestReporter_DispatchKeepaliveRequest_Errors(t *testing.T) {
	tests := []struct {
		name       string
		instanceID []byte
		payload    []byte
		ipaddr     string
		wantErr    error
	}{
		{
			name:       "positive case",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "1.1.1.1",
			wantErr:    nil,
		},
		{
			name:       "positive edge case - some nulls in instance id",
			instanceID: []byte{0xfe, 0x00, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0x00, 0xf0, 0x0d},
			ipaddr:     "1.1.1.1",
			wantErr:    nil,
		},
		{
			name:       "unmatched ip address",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "2.2.2.2",
			wantErr:    reporting.ErrUnknownInstanceID,
		},
		{
			name:       "unknown instance key",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xde, 0xad, 0xbe, 0xef},
			ipaddr:     "1.1.1.1",
			wantErr:    repos.ErrInstanceNotFound,
		},
		{
			name:       "unacceptable payload - length",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			payload:    []byte{0x08, 0xfe, 0xed},
			ipaddr:     "1.1.1.1",
			wantErr:    reporting.ErrInvalidRequestPayload,
		},
	}
	reportedServerIP := "1.1.1.1"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var service *reporting.Service
			var repo repos.ServerRepository

			ctx := context.TODO()
			clockMock := clock.NewMock()

			makeApp(t, fx.Populate(&service, &repo), overrideClock(clockMock))

			// initial heartbeat report
			service.DispatchRequest( // nolint: errcheck
				context.TODO(),
				testutils.PackHeartbeatRequest(tt.instanceID, testutils.GenServerParams()),
				&net.UDPAddr{IP: net.ParseIP(reportedServerIP), Port: 10481},
			)

			clockMock.Add(time.Millisecond)

			// keepalive request in a while
			since := clockMock.Now()
			_, _, err := service.DispatchRequest(
				context.TODO(),
				tt.payload,
				&net.UDPAddr{IP: net.ParseIP(tt.ipaddr), Port: 10481},
			)
			reportedSinceAfter, _ := repo.Filter(ctx, repos.NewServerFilterSet().ActiveAfter(since).WithStatus(ds.Master))
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Len(t, reportedSinceAfter, 0)
			} else {
				require.NoError(t, err)
				assert.Len(t, reportedSinceAfter, 1)
			}
		})
	}
}
