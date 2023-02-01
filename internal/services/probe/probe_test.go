package probe_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/probes"
	prbrepo "github.com/sergeii/swat4master/internal/core/probes/memory"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
)

func TestProbeService_PopMany(t *testing.T) {
	ctx := context.TODO()
	repo := prbrepo.New()
	service := probe.NewService(repo, monitoring.NewMetricService())

	// empty
	empty, err := service.PopMany(ctx, 5)
	assert.NoError(t, err)
	assert.Len(t, empty, 0)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		repo.Add(ctx, probes.New(addr.MustNewFromString(ipaddr, 10480), 10480, probes.GoalDetails)) // nolint: errcheck
	}

	targets, _ := service.PopMany(ctx, 2)
	assert.Len(t, targets, 2)
	assert.Equal(t, "1.1.1.1", targets[0].GetDottedIP())
	assert.Equal(t, "2.2.2.2", targets[1].GetDottedIP())

	targets, _ = service.PopMany(ctx, 2)
	assert.Len(t, targets, 1)
	assert.Equal(t, "3.3.3.3", targets[0].GetDottedIP())

	// exhausted
	targets, _ = service.PopMany(ctx, 2)
	assert.Len(t, targets, 0)
}
