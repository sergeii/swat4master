package serverobserver_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/metrics/observers/serverobserver"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Get(0).(int), args.Error(1) //nolint: forcetypeassert
}

func (m *MockServerRepository) CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[ds.DiscoveryStatus]int), args.Error(1) //nolint: forcetypeassert
}

func (m *MockServerRepository) Filter(ctx context.Context, fs filterset.ServerFilterSet) ([]server.Server, error) {
	args := m.Called(ctx, fs)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).([]server.Server), nil //nolint: forcetypeassert
}

func TestServerObserver_Observe_OK(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	clock := clockwork.NewFakeClock()

	collector := metrics.New()

	countByStatus := map[ds.DiscoveryStatus]int{
		ds.Master: 10,
		ds.Info:   8,
	}
	activeServers := []server.Server{
		serverfactory.Build(
			serverfactory.WithRandomAddress(),
			serverfactory.WithInfo(map[string]string{
				"gametype":   "VIP Escort",
				"numplayers": "0",
			}),
		),
		serverfactory.Build(
			serverfactory.WithRandomAddress(),
			serverfactory.WithInfo(map[string]string{
				"gametype":   "VIP Escort",
				"numplayers": "15",
			}),
		),
		serverfactory.Build(
			serverfactory.WithRandomAddress(),
			serverfactory.WithInfo(map[string]string{
				"gametype":   "CO-OP",
				"numplayers": "5",
			}),
		),
	}

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(37, nil)
	serverRepo.On("CountByStatus", ctx).Return(countByStatus, nil)
	serverRepo.On("Filter", ctx, mock.Anything).Return(activeServers, nil)

	opts := serverobserver.Opts{
		ServerLiveness: time.Hour,
	}
	observer := serverobserver.New(collector, serverRepo, clock, &logger, opts)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.ServerRepositorySize)
	assert.InDelta(t, float64(37), repoSizeValue, 1e-9)

	discoveredServersWithMasterValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("master"))
	assert.InDelta(t, float64(10), discoveredServersWithMasterValue, 1e-9)
	discoveredServersWithInfoValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("info"))
	assert.InDelta(t, float64(8), discoveredServersWithInfoValue, 1e-9)
	discoveredServersWithDetailsValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("details"))
	assert.InDelta(t, float64(0), discoveredServersWithDetailsValue, 1e-9)

	gamePlayersVipEscortValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(15), gamePlayersVipEscortValue, 1e-9)
	gamePlayersCoopValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("CO-OP"))
	assert.InDelta(t, float64(5), gamePlayersCoopValue, 1e-9)
	gamePlayersRdValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("Rapid Deployment"))
	assert.InDelta(t, float64(0), gamePlayersRdValue, 1e-9)

	gameActiveServersVipEscortValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(2), gameActiveServersVipEscortValue, 1e-9)
	gameActiveServersCoopValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("CO-OP"))
	assert.InDelta(t, float64(1), gameActiveServersCoopValue, 1e-9)
	gameActiveServersRdValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("Rapid Deployment"))
	assert.InDelta(t, float64(0), gameActiveServersRdValue, 1e-9)

	gamePlayedServersVipEscortValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(1), gamePlayedServersVipEscortValue, 1e-9)
	gamePlayedServersCoopValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("CO-OP"))
	assert.InDelta(t, float64(1), gamePlayedServersCoopValue, 1e-9)
	gamePlayedServersRdValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("Rapid Deployment"))
	assert.InDelta(t, float64(0), gamePlayedServersRdValue, 1e-9)

	serverRepo.AssertExpectations(t)
	serverRepo.AssertCalled(
		t,
		"Filter",
		ctx,
		mock.MatchedBy(func(fs filterset.ServerFilterSet) bool {
			withStatus, withStatusIsSet := fs.GetWithStatus()
			activeAfter, activeAfterIsSet := fs.GetActiveAfter()
			wantWithStatus := withStatusIsSet && withStatus == ds.Info
			wantActiveAfter := activeAfterIsSet && activeAfter.Equal(clock.Now().Add(-time.Hour))
			return wantWithStatus && wantActiveAfter
		}),
	)
}

func TestServerObserver_Observe_RepoFailure(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	clock := clockwork.NewFakeClock()

	collector := metrics.New()

	serverRepo := new(MockServerRepository)
	serverRepo.On("Count", ctx).Return(0, errors.New("repo error"))
	serverRepo.On("CountByStatus", ctx).Return(map[ds.DiscoveryStatus]int{}, errors.New("repo error"))
	serverRepo.On("Filter", ctx, mock.Anything).Return(nil, errors.New("repo error"))

	opts := serverobserver.Opts{
		ServerLiveness: time.Hour,
	}
	observer := serverobserver.New(collector, serverRepo, clock, &logger, opts)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.ServerRepositorySize)
	assert.InDelta(t, float64(0), repoSizeValue, 1e-9)

	discoveredServersWithMasterValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("master"))
	assert.InDelta(t, float64(0), discoveredServersWithMasterValue, 1e-9)

	gamePlayersVipEscortValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(0), gamePlayersVipEscortValue, 1e-9)

	gameActiveServersVipEscortValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(0), gameActiveServersVipEscortValue, 1e-9)

	gamePlayedServersVipEscortValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("VIP Escort"))
	assert.InDelta(t, float64(0), gamePlayedServersVipEscortValue, 1e-9)

	serverRepo.AssertExpectations(t)
}
