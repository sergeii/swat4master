package finder_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/finder"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/validation"
)

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestFinder_Run_OK(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		DiscoveryRefreshInterval: time.Millisecond * 100,
		DiscoveryRevivalInterval: time.Millisecond * 200,
		DiscoveryRevivalScope:    time.Millisecond * 700,
	}
	app := application.Configure()

	assertTargets := func(wantCount, wantExpired int, wantDetails, wantPorts []string) {
		count, err := app.Probes.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, wantCount, count)

		targets, expired, err := app.Probes.PopMany(ctx, count)
		require.NoError(t, err)
		detailsTargets := make([]string, 0, len(wantDetails))
		portTargets := make([]string, 0, len(wantPorts))
		for _, tgt := range targets {
			switch tgt.GetGoal() {
			case probes.GoalDetails:
				detailsTargets = append(detailsTargets, tgt.GetAddr().String())
			case probes.GoalPort:
				portTargets = append(portTargets, tgt.GetAddr().String())
			}
		}
		assert.Equal(t, wantExpired, expired)
		assert.Equal(t, wantDetails, detailsTargets)
		assert.Equal(t, wantPorts, portTargets)
	}

	info := details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Awesome Server",
		"hostport":    "10580",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "CO-OP",
	})

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.UpdateInfo(info)
	gs1.UpdateDiscoveryStatus(ds.Master)

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateInfo(info)
	gs2.UpdateDiscoveryStatus(ds.Port)

	gs3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.UpdateInfo(info)
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	gs4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.UpdateInfo(info)
	gs4.UpdateDiscoveryStatus(ds.NoDetails)

	gs5, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.UpdateInfo(info)
	gs5.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs6, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.UpdateInfo(info)
	gs6.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	gs7, _ := servers.New(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs7.UpdateInfo(info)
	gs7.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	gs8, _ := servers.New(net.ParseIP("8.8.8.8"), 10480, 10481)
	gs8.UpdateInfo(info)
	gs8.UpdateDiscoveryStatus(ds.Master | ds.PortRetry)

	gs9, _ := servers.New(net.ParseIP("9.9.9.9"), 10480, 10481)
	gs9.UpdateInfo(info)
	gs9.UpdateDiscoveryStatus(ds.Port | ds.PortRetry)

	gs1, _ = app.Servers.AddOrUpdate(ctx, gs1)
	gs2, _ = app.Servers.AddOrUpdate(ctx, gs2)
	gs3, _ = app.Servers.AddOrUpdate(ctx, gs3)
	gs4, _ = app.Servers.AddOrUpdate(ctx, gs4)
	gs5, _ = app.Servers.AddOrUpdate(ctx, gs5)
	gs6, _ = app.Servers.AddOrUpdate(ctx, gs6)
	gs7, _ = app.Servers.AddOrUpdate(ctx, gs7)
	gs8, _ = app.Servers.AddOrUpdate(ctx, gs8)
	gs9, _ = app.Servers.AddOrUpdate(ctx, gs9)

	runner := running.NewRunner(app, cfg)
	runner.Add(finder.Run, ctx)

	<-time.After(time.Millisecond * 150) // 150ms
	// only details timer triggered
	assertTargets(3, 0,
		[]string{"9.9.9.9:10480", "3.3.3.3:10480", "2.2.2.2:10480"}, []string{},
	)

	<-time.After(time.Millisecond * 100) // 250ms
	// details and port timer triggered
	assertTargets(7, 0,
		[]string{"9.9.9.9:10480", "3.3.3.3:10480", "2.2.2.2:10480"},
		[]string{"7.7.7.7:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
	)

	gs3.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs6.ClearDiscoveryStatus(ds.DetailsRetry)
	gs3, _ = app.Servers.AddOrUpdate(ctx, gs3)
	gs6, _ = app.Servers.AddOrUpdate(ctx, gs6)

	<-time.After(time.Millisecond * 200) // 450ms
	// port timer triggered, details triggered twice
	assertTargets(11,
		3,
		[]string{
			"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480",
		},
		[]string{"3.3.3.3:10480", "7.7.7.7:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
	)

	gs2.ClearDiscoveryStatus(ds.Port)
	gs6.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs9.ClearDiscoveryStatus(ds.Port)
	gs2, _ = app.Servers.AddOrUpdate(ctx, gs2)
	gs6, _ = app.Servers.AddOrUpdate(ctx, gs6)
	gs9, _ = app.Servers.AddOrUpdate(ctx, gs9)

	<-time.After(time.Millisecond * 200) // 650ms
	// port timer triggered, details never triggered
	assertTargets(7,
		0,
		[]string{},
		[]string{
			"6.6.6.6:10480", "2.2.2.2:10480", "3.3.3.3:10480",
			"7.7.7.7:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480",
		},
	)

	// bump server
	gs6.UpdateInfo(info)
	gs6, _ = app.Servers.AddOrUpdate(ctx, gs6)
	<-time.After(time.Millisecond * 400) // 700ms

	// all other servers are out of scope
	assertTargets(1, 0, []string{}, []string{"6.6.6.6:10480"})

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

	gs1, _ = app.Servers.AddOrUpdate(ctx, gs1)
	gs2, _ = app.Servers.AddOrUpdate(ctx, gs2)

	runner := running.NewRunner(app, cfg)
	runner.Add(finder.Run, ctx)

	<-time.After(time.Millisecond * 18)

	countAfterManyTicks, _ := app.Probes.Count(ctx)
	assert.Equal(t, 6, countAfterManyTicks)

	actualTargetsAfterManyTicks, _, _ := app.Probes.PopMany(ctx, 6)
	assert.Len(t, actualTargetsAfterManyTicks, 2)

	countAfterPop, _ := app.Probes.Count(ctx)
	assert.Equal(t, 0, countAfterPop)

	cancel()
	runner.WaitQuit()
}
