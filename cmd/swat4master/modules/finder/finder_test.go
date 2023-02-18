package finder_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/finder"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/persistence/memory"
)

func TestFinder_Run_OK(t *testing.T) {
	repos := memory.New()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	assertTargets := func(wantCount, wantExpired int, wantDetails, wantPorts []string) {
		count, err := repos.Probes.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, wantCount, count)

		targets, expired, err := repos.Probes.PopMany(ctx, count)
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

	gs1, _ = repos.Servers.Add(ctx, gs1, servers.OnConflictIgnore)
	gs2, _ = repos.Servers.Add(ctx, gs2, servers.OnConflictIgnore)
	gs3, _ = repos.Servers.Add(ctx, gs3, servers.OnConflictIgnore)
	gs4, _ = repos.Servers.Add(ctx, gs4, servers.OnConflictIgnore)
	gs5, _ = repos.Servers.Add(ctx, gs5, servers.OnConflictIgnore)
	gs6, _ = repos.Servers.Add(ctx, gs6, servers.OnConflictIgnore)
	gs7, _ = repos.Servers.Add(ctx, gs7, servers.OnConflictIgnore)
	gs8, _ = repos.Servers.Add(ctx, gs8, servers.OnConflictIgnore)
	gs9, _ = repos.Servers.Add(ctx, gs9, servers.OnConflictIgnore)

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRefreshInterval:  time.Millisecond * 100,
				DiscoveryRevivalInterval:  time.Millisecond * 200,
				DiscoveryRevivalScope:     time.Millisecond * 700,
				DiscoveryRevivalCountdown: time.Millisecond,
				DiscoveryRevivalPorts:     []int{0},
			}
		}),
		fx.Decorate(func() (servers.Repository, probes.Repository) {
			return repos.Servers, repos.Probes
		}),
		finder.Module,
		fx.NopLogger,
		fx.Invoke(func(*finder.Finder) {}),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

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
	gs3, _ = repos.Servers.Update(ctx, gs3, servers.OnConflictIgnore)
	gs6, _ = repos.Servers.Update(ctx, gs6, servers.OnConflictIgnore)

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
	gs2, _ = repos.Servers.Update(ctx, gs2, servers.OnConflictIgnore)
	gs6, _ = repos.Servers.Update(ctx, gs6, servers.OnConflictIgnore)
	gs9, _ = repos.Servers.Update(ctx, gs9, servers.OnConflictIgnore)

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
	gs6, _ = repos.Servers.Update(ctx, gs6, servers.OnConflictIgnore)
	<-time.After(time.Millisecond * 400) // 700ms

	// all other servers are out of scope
	assertTargets(1, 0, []string{}, []string{"6.6.6.6:10480"})
}

func TestFinder_Run_Expiry(t *testing.T) {
	repos := memory.New()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	gs1, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs1.UpdateDiscoveryStatus(ds.Details | ds.Port)

	gs2, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs2.UpdateDiscoveryStatus(ds.Master | ds.Port)

	gs1, _ = repos.Servers.Add(ctx, gs1, servers.OnConflictIgnore)
	gs2, _ = repos.Servers.Add(ctx, gs2, servers.OnConflictIgnore)

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRefreshInterval:  time.Millisecond * 25,
				DiscoveryRevivalInterval:  time.Millisecond * 25,
				DiscoveryRevivalScope:     time.Second,
				DiscoveryRevivalCountdown: time.Millisecond,
				DiscoveryRevivalPorts:     []int{0},
			}
		}),
		fx.Decorate(func() (servers.Repository, probes.Repository) {
			return repos.Servers, repos.Probes
		}),
		finder.Module,
		fx.NopLogger,
		fx.Invoke(func(*finder.Finder) {}),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	<-time.After(time.Millisecond * 90)

	countAfterManyTicks, _ := repos.Probes.Count(ctx)
	assert.Equal(t, 6, countAfterManyTicks)

	actualTargetsAfterManyTicks, _, _ := repos.Probes.PopMany(ctx, 6)
	assert.Len(t, actualTargetsAfterManyTicks, 2)

	countAfterPop, _ := repos.Probes.Count(ctx)
	assert.Equal(t, 0, countAfterPop)
}
