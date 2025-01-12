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
	return args.Get(0).(int), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) CountByStatus(ctx context.Context) (map[ds.DiscoveryStatus]int, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[ds.DiscoveryStatus]int), args.Error(1) // nolint: forcetypeassert
}

func (m *MockServerRepository) Filter(ctx context.Context, fs filterset.ServerFilterSet) ([]server.Server, error) {
	args := m.Called(ctx, fs)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).([]server.Server), nil // nolint: forcetypeassert
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
	assert.Equal(t, float64(37), repoSizeValue)

	discoveredServersWithMasterValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("master"))
	assert.Equal(t, float64(10), discoveredServersWithMasterValue)
	discoveredServersWithInfoValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("info"))
	assert.Equal(t, float64(8), discoveredServersWithInfoValue)
	discoveredServersWithDetailsValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("details"))
	assert.Equal(t, float64(0), discoveredServersWithDetailsValue)

	gamePlayersVipEscortValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(15), gamePlayersVipEscortValue)
	gamePlayersCoopValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("CO-OP"))
	assert.Equal(t, float64(5), gamePlayersCoopValue)
	gamePlayersRdValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("Rapid Deployment"))
	assert.Equal(t, float64(0), gamePlayersRdValue)

	gameActiveServersVipEscortValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(2), gameActiveServersVipEscortValue)
	gameActiveServersCoopValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("CO-OP"))
	assert.Equal(t, float64(1), gameActiveServersCoopValue)
	gameActiveServersRdValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("Rapid Deployment"))
	assert.Equal(t, float64(0), gameActiveServersRdValue)

	gamePlayedServersVipEscortValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(1), gamePlayedServersVipEscortValue)
	gamePlayedServersCoopValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("CO-OP"))
	assert.Equal(t, float64(1), gamePlayedServersCoopValue)
	gamePlayedServersRdValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("Rapid Deployment"))
	assert.Equal(t, float64(0), gamePlayedServersRdValue)

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
	assert.Equal(t, float64(0), repoSizeValue)

	discoveredServersWithMasterValue := testutil.ToFloat64(collector.GameDiscoveredServers.WithLabelValues("master"))
	assert.Equal(t, float64(0), discoveredServersWithMasterValue)

	gamePlayersVipEscortValue := testutil.ToFloat64(collector.GamePlayers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(0), gamePlayersVipEscortValue)

	gameActiveServersVipEscortValue := testutil.ToFloat64(collector.GameActiveServers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(0), gameActiveServersVipEscortValue)

	gamePlayedServersVipEscortValue := testutil.ToFloat64(collector.GamePlayedServers.WithLabelValues("VIP Escort"))
	assert.Equal(t, float64(0), gamePlayedServersVipEscortValue)

	serverRepo.AssertExpectations(t)
}
