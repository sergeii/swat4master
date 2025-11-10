package components_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/components/prober"
	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
	"github.com/sergeii/swat4master/tests/testapp"
)

func TestProber_Run(t *testing.T) {
	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	app := fx.New(
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Supply(prober.Config{
			PollInterval: time.Millisecond * 100,
			Concurrency:  5,
			ProbeTimeout: time.Millisecond * 50,
			PortOffsets:  []int{1},
		}),
		prober.Module,
		fx.NopLogger,
		fx.Invoke(func(*prober.Component) {}),
		fx.Populate(&serverRepo, &probeRepo),
	)

	var i1 int64
	responses1 := make(chan []byte)
	udp1, cancel1 := gs1.ServerFactory(
		func(_ context.Context, conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
			packet := <-responses1
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			atomic.AddInt64(&i1, 1)
		},
	)
	udpAddr1 := udp1.LocalAddr()
	defer cancel1()

	responses2 := make(chan []byte)
	var i2 int64
	udp2, cancel2 := gs1.ServerFactory(
		func(_ context.Context, conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
			packet := <-responses2
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			atomic.AddInt64(&i2, 1)
		},
	)
	udpAddr2 := udp2.LocalAddr()
	defer cancel2()

	var i3 int64
	udp3, cancel3 := gs1.ServerFactory(
		func(_ context.Context, _ *net.UDPConn, _ *net.UDPAddr, _ []byte) {
			atomic.AddInt64(&i3, 1)
		},
	)
	udpAddr3 := udp3.LocalAddr()
	defer cancel3()

	go func(ctx context.Context, ch chan []byte, gamePort int) {
		packet := []byte(
			fmt.Sprintf(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort"+
					"\\gamevariant\\SWAT 4\\mapname\\Northside Vending\\hostport\\%d\\password\\0"+
					"\\gamever\\1.1\\final\\\\queryid\\1.1",
				gamePort,
			),
		)
		for {
			select {
			case <-ctx.Done():
				return
			case ch <- packet:
			}
		}
	}(ctx, responses1, udpAddr1.Port-1)

	go func(ctx context.Context, ch chan []byte, gamePort int) {
		packet := []byte(
			fmt.Sprintf(
				"\\hostname\\[c=ffff00]WWW.EPiCS.TOP\\numplayers\\8\\maxplayers\\16"+
					"\\gametype\\Barricaded Suspects\\gamevariant\\SWAT 4X\\mapname\\A-Bomb Nightclub"+
					"\\hostport\\%d\\password\\0\\gamever\\1.0\\statsenabled\\0"+
					"\\player_0\\astrfaefgsgdf4g54ezr\\player_1\\Chester\\player_2\\wesaq"+
					"\\player_3\\AJ\\player_4\\Light\\player_5\\Robin\\player_6\\[c=8B008B]infeKtedDicK(VIEW)"+
					"\\player_7\\Acab\\score_0\\0\\score_1\\6\\score_2\\-4\\score_3\\7\\score_4\\11"+
					"\\score_5\\1\\score_6\\0\\score_7\\1\\ping_0\\119\\ping_1\\19\\ping_2\\59\\ping_3\\21"+
					"\\ping_4\\79\\ping_5\\80\\ping_6\\122\\ping_7\\53\\final\\\\queryid\\1.1",
				gamePort,
			),
		)
		for {
			select {
			case <-ctx.Done():
				return
			case ch <- packet:
			}
		}
	}(ctx, responses2, udpAddr2.Port-1)

	info := details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	})

	svr1 := server.MustNewFromAddr(addr.NewForTesting(udpAddr1.IP, udpAddr1.Port-1), udpAddr1.Port)
	svr1.UpdateInfo(info)
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Port)

	svr2 := server.MustNewFromAddr(addr.NewForTesting(udpAddr2.IP, udpAddr2.Port-1), udpAddr2.Port)
	svr2.UpdateInfo(info)
	svr2.UpdateDiscoveryStatus(ds.Master | ds.PortRetry | ds.DetailsRetry)

	svr3 := server.MustNewFromAddr(addr.NewForTesting(udpAddr3.IP, udpAddr3.Port-1), udpAddr3.Port)
	svr3.UpdateInfo(info)
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Port)

	svr4 := serverfactory.BuildRandom()
	svr4.UpdateInfo(info)
	svr4.UpdateDiscoveryStatus(ds.Master)

	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// wait for the prober to spin up
	<-time.After(time.Millisecond * 100)

	svr1, _ = serverRepo.Add(ctx, svr1, repositories.ServerOnConflictIgnore)
	svr2, _ = serverRepo.Add(ctx, svr2, repositories.ServerOnConflictIgnore)
	svr3, _ = serverRepo.Add(ctx, svr3, repositories.ServerOnConflictIgnore)
	svr4, _ = serverRepo.Add(ctx, svr4, repositories.ServerOnConflictIgnore)

	probeRepo.Add(ctx, probe.New(svr1.Addr, svr1.QueryPort, probe.GoalDetails, 2)) // nolint: errcheck
	probeRepo.Add(ctx, probe.New(svr2.Addr, svr2.Addr.Port, probe.GoalPort, 2))    // nolint: errcheck
	probeRepo.Add(ctx, probe.New(svr3.Addr, svr3.QueryPort, probe.GoalDetails, 2)) // nolint: errcheck

	// run a cycle
	<-time.After(time.Millisecond * 175)

	updatedSvr1, _ := serverRepo.Get(ctx, svr1.Addr)
	// has status
	assert.True(t, updatedSvr1.HasDiscoveryStatus(ds.Master|ds.Port|ds.Info|ds.Details))
	assert.Equal(t, "-==MYT Team Svr==-", updatedSvr1.Info.Hostname)
	assert.Equal(t, 16, updatedSvr1.Info.MaxPlayers)
	assert.Equal(t, int64(1), atomic.LoadInt64(&i1))

	assert.Equal(t, "-==MYT Team Svr==-", updatedSvr1.Details.Info.Hostname)
	assert.Equal(t, 16, updatedSvr1.Details.Info.MaxPlayers)
	assert.Equal(t, 0, updatedSvr1.Details.Info.NumPlayers)

	updatedSvr2, _ := serverRepo.Get(ctx, svr2.Addr)
	assert.True(t, updatedSvr2.HasDiscoveryStatus(ds.Master|ds.Port|ds.Info|ds.Details))
	assert.Equal(t, "[c=ffff00]WWW.EPiCS.TOP", updatedSvr2.Info.Hostname)
	assert.Equal(t, int64(1), atomic.LoadInt64(&i2))

	retriedSvr, _ := serverRepo.Get(ctx, svr3.Addr)
	assert.True(t, retriedSvr.HasDiscoveryStatus(ds.Master|ds.Port|ds.DetailsRetry))
	assert.Equal(t, "Swat4 Server", retriedSvr.Info.Hostname)
	assert.Equal(t, int64(1), atomic.LoadInt64(&i3))

	notUpdatedSvr, _ := serverRepo.Get(ctx, svr4.Addr)
	assert.Equal(t, ds.Master, notUpdatedSvr.DiscoveryStatus)
	assert.Equal(t, "Swat4 Server", notUpdatedSvr.Info.Hostname)

	// check probe queue, only 1 probe should be added
	probeCount, _ := probeRepo.Count(ctx)
	require.Equal(t, 1, probeCount)

	retryProbe, _ := probeRepo.Peek(ctx)
	assert.Equal(t, svr3.Addr, retryProbe.Addr)
	assert.Equal(t, svr3.QueryPort, retryProbe.Port)
	assert.Equal(t, probe.GoalDetails, retryProbe.Goal)
}
