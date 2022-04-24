package prober_test

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
	"github.com/sergeii/swat4master/cmd/swat4master/running/prober"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestProber_Run(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.TODO())
	defer cancelCtx()

	cfg := config.Config{
		DiscoveryRefreshInterval: time.Millisecond * 15,
		DiscoveryRevivalInterval: time.Millisecond * 50,
		DiscoveryRevivalScope:    time.Second,
		DiscoveryRevivalPorts:    []int{1, 2, 3, 4},
		ProbeConcurrency:         5,
		ProbePollSchedule:        time.Millisecond,
		ProbeRetries:             2,
		ProbeTimeout:             time.Millisecond * 5,
	}
	app := application.Configure()

	i1 := 0
	udp1, cancelSvr1 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\16\\maxplayers\\16\\gametype\\VIP Escort" +
					"\\gamevariant\\SWAT 4\\mapname\\Northside Vending\\hostport\\10480\\password\\0" +
					"\\gamever\\1.1\\player_0\\AntiRadioSpamBot\\player_1\\AntiRadioSpamBot\\player_2" +
					"\\Dodote\\player_3\\Johnny\\player_4\\play\\player_5\\Tommy\\player_6\\Jericho" +
					"\\player_7\\Daniel\\player_8\\Gery\\player_9\\Forman\\player_10\\Issou\\player_11" +
					"\\Pierce\\player_12\\Osa\\player_13\\t00elitte\\player_14\\sernas\\player_15" +
					"\\|MYT|Svea>M<\\score_0\\0\\score_1\\0\\score_2\\2\\score_3\\10\\score_4\\2\\score_5\\2" +
					"\\score_6\\1\\score_7\\7\\score_8\\1\\score_9\\3\\score_10\\8\\score_11\\15\\score_12\\1" +
					"\\score_13\\11\\score_14\\3\\score_15\\2\\ping_0\\0\\ping_1\\0\\ping_2\\167\\ping_3\\56" +
					"\\ping_4\\128\\ping_5\\36\\ping_6\\35\\ping_7\\34\\ping_8\\23\\ping_9\\18\\ping_10\\15" +
					"\\ping_11\\24\\ping_12\\40\\ping_13\\33\\ping_14\\40\\ping_15\\38\\final\\\\queryid\\1.1",
			)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			i1++
		},
	)
	addr1 := udp1.LocalAddr()
	defer cancelSvr1()

	i2 := 0
	udp2, cancelSvr2 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			i2++
			panic("should not be called")
		},
	)
	addr2 := udp2.LocalAddr()
	defer cancelSvr2()

	i3 := 0
	udp3, cancelSvr3 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\[c=ffff00]WWW.EPiCS.TOP\\numplayers\\8\\maxplayers\\16" +
					"\\gametype\\Barricaded Suspects\\gamevariant\\SWAT 4X\\mapname\\A-Bomb Nightclub" +
					"\\hostport\\10480\\password\\0\\gamever\\1.0\\statsenabled\\0" +
					"\\player_0\\astrfaefgsgdf4g54ezr\\player_1\\Chester\\player_2\\wesaq" +
					"\\player_3\\AJ\\player_4\\Light\\player_5\\Robin\\player_6\\[c=8B008B]infeKtedDicK(VIEW)" +
					"\\player_7\\Acab\\score_0\\0\\score_1\\6\\score_2\\-4\\score_3\\7\\score_4\\11" +
					"\\score_5\\1\\score_6\\0\\score_7\\1\\ping_0\\119\\ping_1\\19\\ping_2\\59\\ping_3\\21" +
					"\\ping_4\\79\\ping_5\\80\\ping_6\\122\\ping_7\\53\\final\\\\queryid\\1.1",
			)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
			i3++
		},
	)
	addr3 := udp3.LocalAddr()
	defer cancelSvr3()

	svr1, err := servers.NewFromAddr(addr.NewForTesting(addr1.IP, addr1.Port-1), addr1.Port)
	require.NoError(t, err)
	svr1.UpdateDiscoveryStatus(ds.Details | ds.Master | ds.Port)

	svr2, err := servers.NewFromAddr(addr.NewForTesting(addr2.IP, addr2.Port-1), addr2.Port)
	require.NoError(t, err)
	svr2.UpdateDiscoveryStatus(ds.PortRetry | ds.DetailsRetry) // has both PortRetry and DetailsRetry status

	svr3, err := servers.NewFromAddr(addr.NewForTesting(addr3.IP, addr3.Port-1), addr3.Port)
	require.NoError(t, err)
	svr3.UpdateDiscoveryStatus(ds.Master)

	app.Servers.AddOrUpdate(ctx, svr1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr2) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr3) // nolint: errcheck

	runner := running.NewRunner(app, cfg)
	runner.Add(prober.Run, ctx)
	runner.Add(finder.Run, ctx)
	time.Sleep(time.Millisecond * 100)

	updatedSvr1, _ := app.Servers.GetByAddr(ctx, svr1.GetAddr())
	assert.Equal(t, ds.Master|ds.Details|ds.Info|ds.Port, updatedSvr1.GetDiscoveryStatus())

	svr1Info := updatedSvr1.GetInfo()
	assert.Equal(t, "-==MYT Team Svr==-", svr1Info.Hostname)
	assert.Equal(t, 16, svr1Info.MaxPlayers)
	assert.Equal(t, 6, i1)

	svr1Details := updatedSvr1.GetDetails()
	assert.Equal(t, "-==MYT Team Svr==-", svr1Details.Info.Hostname)
	assert.Equal(t, 16, svr1Details.Info.MaxPlayers)
	assert.Len(t, svr1Details.Players, 16)
	assert.Nil(t, svr1Details.Objectives)

	notUpdatedSvr2, _ := app.Servers.GetByAddr(ctx, svr2.GetAddr())
	assert.Equal(t, ds.DetailsRetry|ds.PortRetry, notUpdatedSvr2.GetDiscoveryStatus())
	svr2Info := notUpdatedSvr2.GetInfo()
	assert.Equal(t, "", svr2Info.Hostname)
	assert.Equal(t, 0, i2)

	updatedSvr3, _ := app.Servers.GetByAddr(ctx, svr3.GetAddr())
	assert.Equal(t, ds.Master|ds.Info|ds.Details|ds.Port, updatedSvr3.GetDiscoveryStatus())
	svr3Info := updatedSvr3.GetInfo()
	assert.Equal(t, "[c=ffff00]WWW.EPiCS.TOP", svr3Info.Hostname)
	assert.Equal(t, 4, i3) // 1 port probe + 3 details probes

	cancelCtx()
	runner.WaitQuit()
}
