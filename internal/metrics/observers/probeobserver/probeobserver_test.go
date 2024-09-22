package probeobserver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/metrics/observers/probeobserver"
)

type MockProbeRepository struct {
	mock.Mock
	repositories.ProbeRepository
}

func (m *MockProbeRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Get(0).(int), args.Error(1) // nolint: forcetypeassert
}

func TestProbeObserver_Observe_OK(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	collector := metrics.New()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("Count", ctx).Return(37, nil)

	observer := probeobserver.New(collector, probeRepo, &logger)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.ProbeRepositorySize)
	assert.Equal(t, float64(37), repoSizeValue)

	probeRepo.AssertExpectations(t)
}

func TestProbeObserver_Observe_RepoFailure(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()

	collector := metrics.New()

	probeRepo := new(MockProbeRepository)
	probeRepo.On("Count", ctx).Return(0, errors.New("repo failure"))

	observer := probeobserver.New(collector, probeRepo, &logger)
	observer.Observe(ctx, collector)

	repoSizeValue := testutil.ToFloat64(collector.ProbeRepositorySize)
	assert.Equal(t, float64(0), repoSizeValue)

	probeRepo.AssertExpectations(t)
}
