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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reviver"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
)

func TestReviver_Run_OK(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	assertProbes := func(wantCount, wantExpired int, wantPorts []string) {
		count, err := probeRepo.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, wantCount, count)

		probes, expired, err := probeRepo.PopMany(ctx, count)
		require.NoError(t, err)
		detailsProbes := make([]string, 0)
		portProbes := make([]string, 0, len(wantPorts))
		for _, prb := range probes {
			switch prb.Goal {
			case probe.GoalDetails:
				detailsProbes = append(detailsProbes, prb.Addr.String())
			case probe.GoalPort:
				portProbes = append(portProbes, prb.Addr.String())
			}
		}
		assert.Equal(t, wantExpired, expired)
		assert.Equal(t, []string{}, detailsProbes)
		assert.Equal(t, wantPorts, portProbes)
	}

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRevivalInterval:  time.Millisecond * 100,
				DiscoveryRevivalScope:     time.Millisecond * 1000,
				DiscoveryRevivalCountdown: time.Millisecond,
				DiscoveryRevivalPorts:     []int{0},
			}
		}),
		reviver.Module,
		fx.NopLogger,
		fx.Invoke(func(*reviver.Reviver) {}),
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

	gs4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	gs4.UpdateInfo(info, time.Now())
	gs4.UpdateDiscoveryStatus(ds.DetailsRetry)

	gs5 := server.MustNew(net.ParseIP("5.5.5.5"), 10480, 10481)
	gs5.UpdateInfo(info, time.Now())
	gs5.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)

	gs6 := server.MustNew(net.ParseIP("6.6.6.6"), 10480, 10481)
	gs6.UpdateInfo(info, time.Now())
	gs6.UpdateDiscoveryStatus(ds.Master | ds.PortRetry)

	for _, gs := range []*server.Server{&gs1, &gs2, &gs3, &gs4, &gs5, &gs6} {
		*gs, _ = serverRepo.Add(ctx, *gs, repositories.ServerOnConflictIgnore)
	}

	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// let refresher run a cycle
	<-time.After(time.Millisecond * 150)

	// port probes are added
	assertProbes(3, 0,
		[]string{"5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
	)

	// make gs3 non-revivable
	gs3.ClearDiscoveryStatus(ds.Port)
	gs3, _ = serverRepo.Update(ctx, gs3, repositories.ServerOnConflictIgnore)

	// let reviver run another cycle
	<-time.After(time.Millisecond * 100)
	assertProbes(4, 0,
		[]string{"3.3.3.3:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
	)

	// run a couple of cycles, expect some probes to expire
	<-time.After(time.Millisecond * 200)

	assertProbes(8, 4,
		[]string{"3.3.3.3:10480", "5.5.5.5:10480", "4.4.4.4:10480", "1.1.1.1:10480"},
	)

	// make the remaining servers non-revivable
	gs1.UpdateDiscoveryStatus(ds.Port)
	gs4.UpdateDiscoveryStatus(ds.PortRetry)
	gs5.UpdateDiscoveryStatus(ds.Port)

	gs1, _ = serverRepo.Update(ctx, gs1, repositories.ServerOnConflictIgnore)
	gs4, _ = serverRepo.Update(ctx, gs4, repositories.ServerOnConflictIgnore)
	gs5, _ = serverRepo.Update(ctx, gs5, repositories.ServerOnConflictIgnore)

	// run another cycle, expect only gs3 to be revived
	<-time.After(time.Millisecond * 100)

	assertProbes(1, 0, []string{"3.3.3.3:10480"})

	// the remaining server goes out of revival scope
	<-time.After(time.Millisecond * 500)

	assertProbes(4, 4, []string{})
}
