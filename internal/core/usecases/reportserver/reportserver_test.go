package reportserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/reportserver"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories"
	"github.com/sergeii/swat4master/internal/validation"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Get(ctx context.Context, addr addr.Addr) (server.Server, error) {
	args := m.Called(ctx, addr)
	return args.Get(0).(server.Server), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) Update(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server) bool,
) (server.Server, error) {
	args := m.Called(ctx, svr, onConflict)
	return svr, args.Error(0)
}

func (m *MockServerRepository) AddOrUpdate(
	ctx context.Context,
	svr server.Server,
	onConflict func(*server.Server),
) (server.Server, error) {
	args := m.Called(ctx, svr, onConflict)
	return svr, args.Error(0)
}

type MockInstanceRepository struct {
	mock.Mock
	repositories.InstanceRepository
}

func (m *MockInstanceRepository) Add(ctx context.Context, inst instance.Instance) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

type MockProbeRepository struct {
	mock.Mock
	repositories.ProbeRepository
}

func (m *MockProbeRepository) Add(ctx context.Context, prb probe.Probe) error {
	args := m.Called(ctx, prb)
	return args.Error(0)
}

func TestReportServerUseCase_ReportNewServer(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	clock := clockwork.NewFakeClock()
	validate := validation.MustNew()

	clock.Advance(time.Second)
	passedTime := clock.Now()

	svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)
	svrQueryPort := 10481
	svrParams := testutils.GenServerParams()
	svrInfo, _ := details.NewInfoFromParams(svrParams)

	serverRepo := new(MockServerRepository)
	serverRepo.On("Get", ctx, svrAddr).Return(server.Blank, repositories.ErrServerNotFound)
	serverRepo.On("AddOrUpdate", ctx, mock.Anything, mock.Anything).Return(nil)
	serverRepo.On("Update", ctx, mock.Anything, mock.Anything).Return(nil)

	instanceRepo := new(MockInstanceRepository)
	instanceRepo.On("Add", ctx, mock.Anything).Return(nil)

	probeRepo := new(MockProbeRepository)
	probeRepo.On("Add", ctx, mock.Anything).Return(nil)

	uc := reportserver.New(serverRepo, instanceRepo, probeRepo, validate, clock, &logger)
	req := reportserver.NewRequest(svrAddr, svrQueryPort, "foo", svrParams)
	err := uc.Execute(ctx, req)
	assert.NoError(t, err)

	serverRepo.AssertCalled(t, "Get", ctx, svrAddr)
	serverRepo.AssertCalled(
		t,
		"AddOrUpdate",
		ctx,
		mock.MatchedBy(func(createdServer server.Server) bool {
			hasAddr := createdServer.Addr == svrAddr
			hasQueryPort := createdServer.QueryPort == svrQueryPort
			hasStatus := createdServer.HasDiscoveryStatus(ds.Master | ds.Info)
			hasInfo := createdServer.Info == svrInfo
			hasRefreshedAt := createdServer.RefreshedAt.Equal(passedTime)
			return hasAddr && hasQueryPort && hasStatus && hasInfo && hasRefreshedAt
		}),
		mock.Anything,
	)

	instanceRepo.AssertCalled(
		t,
		"Add",
		ctx,
		mock.MatchedBy(func(createdInstance instance.Instance) bool {
			hasAddr := createdInstance.Addr == svrAddr
			hasID := createdInstance.ID == "foo"
			return hasAddr && hasID
		}),
	)

	probeRepo.AssertCalled(
		t,
		"Add",
		ctx,
		mock.MatchedBy(func(createdProbe probe.Probe) bool {
			hasAddr := createdProbe.Addr == svrAddr
			hasPort := createdProbe.Port == svrAddr.Port
			hasGoal := createdProbe.Goal == probe.GoalPort
			return hasAddr && hasPort && hasGoal
		}),
	)

	serverRepo.AssertCalled(
		t,
		"Update",
		ctx,
		mock.MatchedBy(func(updatedServer server.Server) bool {
			hasStatus := updatedServer.HasDiscoveryStatus(ds.Master | ds.Info | ds.PortRetry)
			return hasStatus
		}),
		mock.Anything,
	)
}

func TestReportServerUseCase_ReportExistingServer(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	validate := validation.MustNew()

	tests := []struct {
		name            string
		discoveryStatus ds.DiscoveryStatus
		wantProbe       bool
	}{
		{
			"server has no expected discovery status",
			ds.Master | ds.Info,
			true,
		},
		{
			"server has Port discovery status",
			ds.Master | ds.Info | ds.Port,
			false,
		},
		{
			"server has PortRetry discovery status",
			ds.Master | ds.Info | ds.PortRetry,
			false,
		},
		{
			"server has Port and PortRetry discovery status",
			ds.Port | ds.PortRetry,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			now := clock.Now()

			svrPlayers := []map[string]string{
				{
					"player": "Player1", "score": "10", "ping": "103", "team": "0",
				},
				{
					"player": "Player2", "score": "0", "ping": "44", "team": "0",
				},
			}
			svrParams := testutils.GenExtraServerParams(map[string]string{"mapname": "A-Bomb Nightclub"})
			svrDetails := details.MustNewDetailsFromParams(svrParams, svrPlayers, nil)

			svr := factories.BuildRandomServer()
			svr.UpdateDetails(svrDetails, now)
			svr.UpdateDiscoveryStatus(tt.discoveryStatus)

			serverRepo := new(MockServerRepository)
			serverRepo.On("Get", ctx, svr.Addr).Return(svr, nil)
			serverRepo.On("AddOrUpdate", ctx, mock.Anything, mock.Anything).Return(nil)
			serverRepo.On("Update", ctx, mock.Anything, mock.Anything).Return(nil)

			instanceRepo := new(MockInstanceRepository)
			instanceRepo.On("Add", ctx, mock.Anything).Return(nil)

			probeRepo := new(MockProbeRepository)
			probeRepo.On("Add", ctx, mock.Anything).Return(nil)

			clock.Advance(time.Second)
			passedTime := clock.Now()

			updatedParams := testutils.GenExtraServerParams(map[string]string{"mapname": "The Wolcott Projects"})

			uc := reportserver.New(serverRepo, instanceRepo, probeRepo, validate, clock, &logger)
			req := reportserver.NewRequest(svr.Addr, svr.QueryPort, "foo", updatedParams)
			err := uc.Execute(ctx, req)
			assert.NoError(t, err)

			serverRepo.AssertCalled(t, "Get", ctx, svr.Addr)
			serverRepo.AssertCalled(
				t,
				"AddOrUpdate",
				ctx,
				mock.MatchedBy(func(updatedSvr server.Server) bool {
					hasAddr := updatedSvr.Addr == svr.Addr
					hasQueryPort := updatedSvr.QueryPort == svr.QueryPort
					hasStatus := updatedSvr.HasDiscoveryStatus(tt.discoveryStatus)
					hasUpdatedInfo := updatedSvr.Info.MapName == "The Wolcott Projects"
					hasRefreshedAt := updatedSvr.RefreshedAt.Equal(passedTime)
					return hasAddr && hasQueryPort && hasStatus && hasUpdatedInfo && hasRefreshedAt
				}),
				mock.Anything,
			)

			instanceRepo.AssertCalled(
				t,
				"Add",
				ctx,
				mock.MatchedBy(func(createdInstance instance.Instance) bool {
					hasAddr := createdInstance.Addr == svr.Addr
					hasID := createdInstance.ID == "foo"
					return hasAddr && hasID
				}),
			)

			if tt.wantProbe {
				probeRepo.AssertCalled(
					t,
					"Add",
					ctx,
					mock.MatchedBy(func(createdProbe probe.Probe) bool {
						hasAddr := createdProbe.Addr == svr.Addr
						hasPort := createdProbe.Port == svr.Addr.Port
						hasGoal := createdProbe.Goal == probe.GoalPort
						return hasAddr && hasPort && hasGoal
					}),
				)
				serverRepo.AssertCalled(
					t,
					"Update",
					ctx,
					mock.MatchedBy(func(updatedServer server.Server) bool {
						hasStatus := updatedServer.HasDiscoveryStatus(tt.discoveryStatus | ds.PortRetry)
						return hasStatus
					}),
					mock.Anything,
				)
			} else {
				probeRepo.AssertNotCalled(t, "Add", mock.Anything, mock.Anything)
				serverRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestReportServerUseCase_InvalidFields(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	validate := validation.MustNew()
	clock := clockwork.NewFakeClock()

	tests := []struct {
		name   string
		params map[string]string
	}{
		{
			"missing required field (hostname)",
			map[string]string{
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
			},
		},
		{
			"invalid numeric value (hostport)",
			map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "bar",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svrAddr := addr.MustNewFromDotted("1.1.1.1", 10480)
			svrQueryPort := 10481

			serverRepo := new(MockServerRepository)
			serverRepo.On("Get", ctx, svrAddr).Return(server.Blank, repositories.ErrServerNotFound)

			instanceRepo := new(MockInstanceRepository)
			probeRepo := new(MockProbeRepository)

			uc := reportserver.New(serverRepo, instanceRepo, probeRepo, validate, clock, &logger)
			req := reportserver.NewRequest(svrAddr, svrQueryPort, "foo", tt.params)
			err := uc.Execute(ctx, req)
			assert.ErrorIs(t, err, reportserver.ErrInvalidRequestPayload)

			serverRepo.AssertCalled(t, "Get", ctx, svrAddr)
			serverRepo.AssertNotCalled(t, "AddOrUpdate", mock.Anything, mock.Anything, mock.Anything)
			instanceRepo.AssertNotCalled(t, "Add", mock.Anything, mock.Anything)
			probeRepo.AssertNotCalled(t, "Add", mock.Anything, mock.Anything)
			serverRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
