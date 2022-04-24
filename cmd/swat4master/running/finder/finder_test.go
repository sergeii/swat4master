package finder_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/finder"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
)

func TestFinder_Run_OK(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		DiscoveryRefreshInterval: time.Millisecond * 15,
		DiscoveryRevivalInterval: time.Millisecond * 25,
		DiscoveryRevivalScope:    time.Second,
	}
	app := application.Configure()

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.UpdateDiscoveryStatus(ds.Master)

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateDiscoveryStatus(ds.Port)

	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	gs4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	gs5, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs6, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	gs7, _ := servers.New(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	gs8, _ := servers.New(net.ParseIP("8.8.8.8"), 10480, 10481)
	gs8.UpdateDiscoveryStatus(ds.Master | ds.PortRetry)

	gs9, _ := servers.New(net.ParseIP("9.9.9.9"), 10480, 10481)
	gs9.UpdateDiscoveryStatus(ds.Port | ds.PortRetry)

	app.Servers.AddOrUpdate(ctx, gs1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs2) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs3) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs4) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs5) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs6) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs7) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs8) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs9) // nolint: errcheck

	runner := running.NewRunner(app, cfg)
	runner.Add(finder.Run, ctx)
	time.Sleep(time.Millisecond * 20)

	countAfter1stTick, _ := app.Probes.Count(ctx)
	assert.Equal(t, 3, countAfter1stTick)

	targetsAfter1stTick, _, _ := app.Probes.PopMany(ctx, 3)
	detailsAfter1stTick := make([]string, 0, 3)
	portsAfter1stTick := make([]string, 0)
	for _, tgt := range targetsAfter1stTick {
		switch tgt.GetGoal() {
		case probes.GoalDetails:
			detailsAfter1stTick = append(detailsAfter1stTick, tgt.GetAddr().String())
		case probes.GoalPort:
			portsAfter1stTick = append(portsAfter1stTick, tgt.GetAddr().String())
		}
	}
	assert.Equal(t, []string{"9.9.9.9:10480", "3.3.3.3:10480", "2.2.2.2:10480"}, detailsAfter1stTick)
	assert.Len(t, portsAfter1stTick, 0)

	gs3.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs6.ClearDiscoveryStatus(ds.DetailsRetry)
	app.Servers.AddOrUpdate(ctx, gs3) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs6) // nolint: errcheck

	time.Sleep(time.Millisecond * 15)

	countAfter2ndTick, _ := app.Probes.Count(ctx)
	assert.Equal(t, 7, countAfter2ndTick)

	targetsAfter2ndTick, _, _ := app.Probes.PopMany(ctx, 7)
	detailsAfter2ndTick := make([]string, 0, 3)
	portsAfter2ndTick := make([]string, 0, 4)
	for _, tgt := range targetsAfter2ndTick {
		switch tgt.GetGoal() {
		case probes.GoalDetails:
			detailsAfter2ndTick = append(detailsAfter2ndTick, tgt.GetAddr().String())
		case probes.GoalPort:
			portsAfter2ndTick = append(portsAfter2ndTick, tgt.GetAddr().String())
		}
	}
	assert.Equal(t, []string{"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480"}, detailsAfter2ndTick)
	assert.Equal(t, []string{"7.7.7.7:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"}, portsAfter2ndTick)

	gs2.ClearDiscoveryStatus(ds.Port)
	gs6.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs9.ClearDiscoveryStatus(ds.Port)
	app.Servers.AddOrUpdate(ctx, gs2) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs6) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs9) // nolint: errcheck

	time.Sleep(time.Millisecond * 15)
	countAfterManyTicks, _ := app.Probes.Count(ctx)
	assert.Equal(t, 5, countAfterManyTicks)

	targetsAfter3rdTick, _, _ := app.Probes.PopMany(ctx, 5)
	detailsAfter3rdTick := make([]string, 0, 2)
	portsAfter3rdTick := make([]string, 0)
	for _, tgt := range targetsAfter3rdTick {
		switch tgt.GetGoal() {
		case probes.GoalDetails:
			detailsAfter3rdTick = append(detailsAfter3rdTick, tgt.GetAddr().String())
		case probes.GoalPort:
			portsAfter3rdTick = append(portsAfter3rdTick, tgt.GetAddr().String())
		}
	}
	assert.Len(t, detailsAfter3rdTick, 0)
	assert.Equal(
		t,
		[]string{"3.3.3.3:10480", "7.7.7.7:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
		portsAfter3rdTick,
	)

	cancel()
	runner.WaitQuit()
}

func TestFinder_Run_Expiry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		DiscoveryRefreshInterval: time.Millisecond * 5,
		DiscoveryRevivalInterval: time.Millisecond * 5,
		DiscoveryRevivalScope:    time.Second,
	}
	app := application.Configure()

	gs1, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs1.UpdateDiscoveryStatus(ds.Details | ds.Port)

	gs2, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs2.UpdateDiscoveryStatus(ds.Master | ds.Port)

	app.Servers.AddOrUpdate(ctx, gs1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, gs2) // nolint: errcheck

	runner := running.NewRunner(app, cfg)
	runner.Add(finder.Run, ctx)

	time.Sleep(time.Millisecond * 18)

	countAfterManyTicks, _ := app.Probes.Count(ctx)
	assert.Equal(t, 6, countAfterManyTicks)

	actualTargetsAfterManyTicks, _, _ := app.Probes.PopMany(ctx, 6)
	assert.Len(t, actualTargetsAfterManyTicks, 2)

	countAfterPop, _ := app.Probes.Count(ctx)
	assert.Equal(t, 0, countAfterPop)

	cancel()
	runner.WaitQuit()
}
