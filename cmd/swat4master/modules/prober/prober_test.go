package prober_test

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/finder"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/prober"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/persistence/memory"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

func TestProber_Run(t *testing.T) {
	clockMock := clock.NewMock()
	repos := memory.New(clockMock)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var i1 int64
	responses1 := make(chan []byte)
	udp1, cancelSvr1 := gs1.ServerFactory(
		func(_ context.Context, conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
			packet := <-responses1
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			atomic.AddInt64(&i1, 1)
		},
	)
	addr1 := udp1.LocalAddr()
	defer cancelSvr1()

	var i2 int64
	udp2, cancelSvr2 := gs1.ServerFactory(
		func(_ context.Context, _ *net.UDPConn, _ *net.UDPAddr, _ []byte) {
			atomic.AddInt64(&i2, 1)
			panic("should not be called")
		},
	)
	addr2 := udp2.LocalAddr()
	defer cancelSvr2()

	responses3 := make(chan []byte)
	var i3 int64
	udp3, cancelSvr3 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := <-responses3
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			atomic.AddInt64(&i3, 1)
		},
	)
	addr3 := udp3.LocalAddr()
	defer cancelSvr3()

	info := details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	})

	svr1, err := servers.NewFromAddr(addr.NewForTesting(addr1.IP, addr1.Port-1), addr1.Port)
	require.NoError(t, err)
	svr1.UpdateInfo(info, clockMock.Now())
	svr1.UpdateDiscoveryStatus(ds.Details | ds.Master | ds.Port)
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
	}(ctx, responses1, svr1.GetGamePort())

	svr2, err := servers.NewFromAddr(addr.NewForTesting(addr2.IP, addr2.Port-1), addr2.Port)
	require.NoError(t, err)
	svr2.UpdateInfo(info, clockMock.Now())
	svr2.UpdateDiscoveryStatus(ds.PortRetry | ds.DetailsRetry) // has both PortRetry and DetailsRetry status

	svr3, err := servers.NewFromAddr(addr.NewForTesting(addr3.IP, addr3.Port-1), addr3.Port)
	require.NoError(t, err)
	svr3.UpdateInfo(info, clockMock.Now())
	svr3.UpdateDiscoveryStatus(ds.Master)
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
	}(ctx, responses3, svr3.GetGamePort())

	repos.Servers.Add(ctx, svr1, servers.OnConflictIgnore) // nolint: errcheck
	clockMock.Add(time.Millisecond)

	repos.Servers.Add(ctx, svr2, servers.OnConflictIgnore) // nolint: errcheck
	clockMock.Add(time.Millisecond)

	repos.Servers.Add(ctx, svr3, servers.OnConflictIgnore) // nolint: errcheck
	clockMock.Add(time.Millisecond)

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				DiscoveryRefreshInterval: time.Millisecond * 150,
				DiscoveryRevivalInterval: time.Millisecond * 500,
				DiscoveryRevivalScope:    time.Second,
				DiscoveryRevivalPorts:    []int{1},
				ProbeConcurrency:         5,
				ProbePollSchedule:        time.Millisecond * 50, // speed up the test
				ProbeRetries:             2,
				ProbeTimeout:             time.Millisecond * 100,
			}
		}),
		fx.Decorate(func() clock.Clock { return clockMock }),
		fx.Decorate(func() servers.Repository {
			return repos.Servers
		}),
		prober.Module,
		finder.Module,
		fx.NopLogger,
		fx.Invoke(func(*finder.Finder, *prober.Prober) {}),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()
	runtime.Gosched()

	for i := 0; i < 100; i++ {
		clockMock.Add(time.Millisecond * 10)
	}

	updatedSvr1, _ := repos.Servers.Get(ctx, svr1.GetAddr())
	assert.Equal(t, ds.Master|ds.Details|ds.Info|ds.Port, updatedSvr1.GetDiscoveryStatus())

	svr1Info := updatedSvr1.GetInfo()
	assert.Equal(t, "-==MYT Team Svr==-", svr1Info.Hostname)
	assert.Equal(t, 16, svr1Info.MaxPlayers)
	assert.Equal(t, int64(6), atomic.LoadInt64(&i1))

	svr1Details := updatedSvr1.GetDetails()
	assert.Equal(t, "-==MYT Team Svr==-", svr1Details.Info.Hostname)
	assert.Equal(t, 16, svr1Details.Info.MaxPlayers)
	assert.Equal(t, 0, svr1Details.Info.NumPlayers)

	notUpdatedSvr2, _ := repos.Servers.Get(ctx, svr2.GetAddr())
	assert.Equal(t, ds.DetailsRetry|ds.PortRetry, notUpdatedSvr2.GetDiscoveryStatus())
	svr2Info := notUpdatedSvr2.GetInfo()
	assert.Equal(t, "Swat4 Server", svr2Info.Hostname)
	assert.Equal(t, int64(0), atomic.LoadInt64(&i2))

	updatedSvr3, _ := repos.Servers.Get(ctx, svr3.GetAddr())
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.Port, updatedSvr3.GetDiscoveryStatus())
	svr3Info := updatedSvr3.GetInfo()
	assert.Equal(t, "[c=ffff00]WWW.EPiCS.TOP", svr3Info.Hostname)
	assert.Equal(t, int64(4), atomic.LoadInt64(&i3)) // 1 port probe + 3 details probes
}
