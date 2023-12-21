package modules_test

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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/refresher"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

func TestRefresher_Run_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	assertProbes := func(wantCount, wantExpired int, wantDetails []string) {
		count, err := probeRepo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, wantCount, count)

		probes, expired, err := probeRepo.PopMany(ctx, count)
		require.NoError(t, err)
		detailsProbes := make([]string, 0, len(wantDetails))
		portProbes := make([]string, 0)
		for _, prb := range probes {
			switch prb.Goal {
			case probe.GoalDetails:
				detailsProbes = append(detailsProbes, prb.Addr.String())
			case probe.GoalPort:
				portProbes = append(portProbes, prb.Addr.String())
			}
		}
		assert.Equal(t, wantExpired, expired)
		assert.Equal(t, wantDetails, detailsProbes)
		assert.Equal(t, []string{}, portProbes)
	}

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRefreshInterval: time.Millisecond * 100,
			}
		}),
		refresher.Module,
		fx.NopLogger,
		fx.Invoke(func(*refresher.Refresher) {}),
		fx.Populate(&serverRepo, &probeRepo),
	)

	info := details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Awesome Server",
		"hostport":    "10580",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "CO-OP",
	})

	gs1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.UpdateInfo(info, time.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)

	gs2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateInfo(info, time.Now())
	gs2.UpdateDiscoveryStatus(ds.Port)

	gs3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	gs3.UpdateInfo(info, time.Now())
	gs3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Port)

	gs4 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs4.UpdateInfo(info, time.Now())
	gs4.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs5 := server.MustNew(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs5.UpdateInfo(info, time.Now())
	gs5.UpdateDiscoveryStatus(ds.Port | ds.Details | ds.DetailsRetry)

	gs6 := server.MustNew(net.ParseIP("7.7.7.7"), 10480, 10481)
	gs6.UpdateInfo(info, time.Now())
	gs6.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	gs7 := server.MustNew(net.ParseIP("9.9.9.9"), 10480, 10481)
	gs7.UpdateInfo(info, time.Now())
	gs7.UpdateDiscoveryStatus(ds.Port | ds.PortRetry)

	for _, gs := range []*server.Server{&gs1, &gs2, &gs3, &gs4, &gs5, &gs6, &gs7} {
		*gs, _ = serverRepo.Add(ctx, *gs, repositories.ServerOnConflictIgnore)
	}

	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// let refresher run a cycle
	<-time.After(time.Millisecond * 150)

	// details probes are added
	assertProbes(3, 0,
		[]string{"9.9.9.9:10480", "3.3.3.3:10480", "2.2.2.2:10480"},
	)

	// clear the server's refreshable status, so that it doesn't get picked up again
	gs3.ClearDiscoveryStatus(ds.Details | ds.Port)
	gs3, _ = serverRepo.Update(ctx, gs3, repositories.ServerOnConflictIgnore)

	// clear the retry status, so it's actually picked up
	gs5.ClearDiscoveryStatus(ds.DetailsRetry)
	gs5, _ = serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore)

	// let refresher run another cycle
	<-time.After(time.Millisecond * 100)

	assertProbes(3,
		0,
		[]string{
			"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480",
		},
	)

	// run a couple of cycles, expect some probes to expire
	<-time.After(time.Millisecond * 200)

	assertProbes(6,
		3,
		[]string{
			"6.6.6.6:10480", "9.9.9.9:10480", "2.2.2.2:10480",
		},
	)

	// make the remaining servers non-refreshable
	gs2.ClearDiscoveryStatus(ds.Port)
	gs5.ClearDiscoveryStatus(ds.Port)
	gs7.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs2, _ = serverRepo.Update(ctx, gs2, repositories.ServerOnConflictIgnore)
	gs5, _ = serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore)
	gs7, _ = serverRepo.Update(ctx, gs7, repositories.ServerOnConflictIgnore)

	// run another cycle, expect no probes
	<-time.After(time.Millisecond * 100)

	assertProbes(0, 0, []string{})
}
