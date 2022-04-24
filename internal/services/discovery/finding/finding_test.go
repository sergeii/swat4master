package finding_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/probes"
	prbrepo "github.com/sergeii/swat4master/internal/core/probes/memory"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
)

func TestFindingService_DiscoverDetails(t *testing.T) {
	ctx := context.TODO()
	queue := prbrepo.New()
	probeSrv := probe.NewService(queue, monitoring.NewMetricService())
	finder := finding.NewService(probeSrv)

	deadline := time.Now().Add(time.Millisecond * 10)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverDetails(ctx, addr.MustNewFromString(ipaddr, 10480), 10481, deadline)
		assert.NoError(t, err)
	}

	t1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())
	assert.Equal(t, probes.GoalDetails, t1.GetGoal())
	assert.Equal(t, 10481, t1.GetPort())

	time.Sleep(time.Millisecond * 15)

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}

func TestFindingService_DiscoverPort(t *testing.T) {
	ctx := context.TODO()
	queue := prbrepo.New()
	probeSrv := probe.NewService(queue, monitoring.NewMetricService())
	finder := finding.NewService(probeSrv)

	countdown := time.Now().Add(time.Millisecond * 5)
	deadline := time.Now().Add(time.Millisecond * 15)

	for _, ipaddr := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		err := finder.DiscoverPort(ctx, addr.MustNewFromString(ipaddr, 10480), countdown, deadline)
		assert.NoError(t, err)
	}

	_, err := queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrTargetIsNotReady)

	time.Sleep(time.Millisecond * 5)

	t1, _ := queue.Pop(ctx)
	assert.Equal(t, "1.1.1.1", t1.GetDottedIP())
	assert.Equal(t, probes.GoalPort, t1.GetGoal())
	assert.Equal(t, 10480, t1.GetPort())

	time.Sleep(time.Millisecond * 5)

	t2, _ := queue.Pop(ctx)
	assert.Equal(t, "2.2.2.2", t2.GetDottedIP())
	assert.Equal(t, probes.GoalPort, t2.GetGoal())
	assert.Equal(t, 10480, t2.GetPort())

	time.Sleep(time.Millisecond * 10)

	_, err = queue.Pop(ctx)
	assert.ErrorIs(t, err, probes.ErrQueueIsEmpty)
}
