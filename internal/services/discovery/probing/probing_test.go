package probing_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/probes"
	prbrepo "github.com/sergeii/swat4master/internal/core/probes/memory"
	"github.com/sergeii/swat4master/internal/core/servers"
	svrrepo "github.com/sergeii/swat4master/internal/core/servers/memory"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/services/discovery/probing"
	"github.com/sergeii/swat4master/internal/services/monitoring"
	"github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

type ResponseFunc func(context.Context, *net.UDPConn, *net.UDPAddr, []byte)

func makeService(opts ...probing.Option) (
	probes.Repository,
	servers.Repository,
	*probing.Service,
) {
	svrRepo := svrrepo.New()
	queue := prbrepo.New()
	metrics := monitoring.NewMetricService()
	probeSrv := probe.NewService(queue, metrics)
	svc := probing.NewService(svrRepo, probeSrv, metrics, opts...)
	return queue, svrRepo, svc
}

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestProbingService_Probe_UnknownGoalType(t *testing.T) {
	_, _, svc := makeService()
	target := probes.New(addr.MustNewFromString("1.1.1.1", 10480), 10481, probes.Goal(10))
	err := svc.Probe(context.TODO(), target)
	assert.ErrorIs(t, err, probing.ErrUnknownGoalType)
}

func TestProbingService_ProbeDetails_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"fresh server is updated",
			ds.New,
			ds.Details | ds.Info,
		},
		{
			"server is updated",
			ds.Details | ds.Master,
			ds.Details | ds.Master | ds.Info,
		},
		{
			"retried server is updated",
			ds.Master | ds.Details | ds.DetailsRetry,
			ds.Master | ds.Details | ds.Info,
		},
		{
			"dead server is updated",
			ds.NoDetails,
			ds.Details | ds.Info,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(probing.WithProbeTimeout(time.Millisecond * 10))

			responses := make(chan []byte)
			go func() {
				responses <- []byte(
					"\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10880" +
						"\\password\\false\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\153" +
						"\\timespecial\\0\\obj_Neutralize_All_Enemies\\1\\obj_Rescue_All_Hostages\\1" +
						"\\obj_Rescue_Rosenstein\\1\\obj_Rescue_Fillinger\\1\\obj_Rescue_Victims\\1" +
						"\\obj_Neutralize_Alice\\1\\tocreports\\8/13\\weaponssecured\\4/6\\queryid\\1\\final\\",
				)
			}()
			udp, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-1), svrAddr.Port)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			repo.AddOrUpdate(ctx, svr) // nolint: errcheck

			target := probes.New(svr.GetAddr(), svrAddr.Port, probes.GoalDetails)

			probeErr := service.Probe(ctx, target)
			require.NoError(t, probeErr)

			queueSize, _ := queue.Count(ctx)
			assert.Equal(t, 0, queueSize)

			updatedSvr, _ := repo.GetByAddr(ctx, svr.GetAddr())
			assert.Equal(t, tt.wantStatus, updatedSvr.GetDiscoveryStatus())

			updatedInfo := updatedSvr.GetInfo()
			assert.Equal(t, "-==MYT Co-op Svr==-", updatedInfo.Hostname)
			assert.Equal(t, "4/6", updatedInfo.WeaponsSecured)

			updatedDetails := updatedSvr.GetDetails()
			assert.Equal(t, "-==MYT Co-op Svr==-", updatedDetails.Info.Hostname)
			assert.Equal(t, "4/6", updatedDetails.Info.WeaponsSecured)
			assert.Len(t, updatedDetails.Objectives, 6)
			assert.Len(t, updatedDetails.Players, 0)
		})
	}
}

func TestProbingService_ProbeDetails_RetryOnError(t *testing.T) {
	tests := []struct {
		name         string
		initStatus   ds.DiscoveryStatus
		serverExists bool
		respFactory  ResponseFunc
		wantStatus   ds.DiscoveryStatus
		wantErr      error
	}{
		{
			"positive case for existing server",
			ds.Details,
			true,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16" +
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
						"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.Details | ds.Info,
			nil,
		},
		{
			"good response for new server",
			ds.NoStatus,
			false,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16" +
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
						"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
		{
			"timeout for existing server",
			ds.Details | ds.Info,
			true,
			func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {},
			ds.Details | ds.Info | ds.DetailsRetry,
			probing.ErrProbeRetried,
		},
		{
			"timeout for new server",
			ds.NoStatus,
			false,
			func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
		{
			"no valid response for existing server",
			ds.Master | ds.Details | ds.Info,
			true,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.Master | ds.Details | ds.Info | ds.DetailsRetry,
			probing.ErrProbeRetried,
		},
		{
			"no valid response for new server",
			ds.NoStatus,
			false,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithMaxRetries(3),
			)

			udp, cancel := gs1.ServerFactory(tt.respFactory)
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-1), svrAddr.Port)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			if tt.serverExists {
				repo.AddOrUpdate(ctx, svr) // nolint: errcheck
			}

			target := probes.New(svr.GetAddr(), svrAddr.Port, probes.GoalDetails)
			probeErr := service.Probe(ctx, target)

			queueSize, _ := queue.Count(ctx)

			updatedSvr, getErr := repo.GetByAddr(ctx, svr.GetAddr())
			updatedInfo := updatedSvr.GetInfo()

			if tt.wantErr != nil {
				require.ErrorIs(t, probeErr, tt.wantErr)
			} else {
				require.NoError(t, probeErr)
			}

			if !tt.serverExists { // nolint: gocritic
				require.ErrorIs(t, getErr, servers.ErrServerNotFound)
				assert.Equal(t, 0, queueSize)
			} else if tt.wantErr != nil {
				assert.Equal(t, tt.wantStatus, updatedSvr.GetDiscoveryStatus())
				require.Equal(t, 1, queueSize)
				require.NoError(t, getErr)
				retry, _ := queue.PopAny(ctx)
				assert.Equal(t, 1, retry.GetRetries())
				assert.Equal(t, target.GetAddr(), retry.GetAddr())
				assert.Equal(t, target.GetPort(), retry.GetPort())
			} else {
				require.NoError(t, getErr)
				assert.Equal(t, 0, queueSize)
				assert.Equal(t, "-==MYT Team Svr==-", updatedInfo.Hostname)
			}
		})
	}
}

func TestProbingService_ProbeDetails_OutOfRetries(t *testing.T) {
	tests := []struct {
		name         string
		initStatus   ds.DiscoveryStatus
		serverExists bool
		wantStatus   ds.DiscoveryStatus
	}{
		{
			"server is not created",
			ds.NoStatus,
			false,
			ds.NoStatus,
		},
		{
			"server is updated",
			ds.Details | ds.DetailsRetry | ds.Info,
			true,
			ds.NoDetails,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithMaxRetries(1),
			)

			udp, cancel := gs1.ServerFactory(func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {})
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-1), svrAddr.Port)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			if tt.serverExists {
				repo.AddOrUpdate(ctx, svr) // nolint: errcheck
			}

			target := probes.New(svr.GetAddr(), svrAddr.Port, probes.GoalDetails)

			// pre-increment retry count
			_, incremented := target.IncRetries(1)
			require.True(t, incremented)

			probeErr := service.Probe(ctx, target)

			queueSize, _ := queue.Count(ctx)
			require.Equal(t, 0, queueSize)

			maybeUpdatedServer, getErr := repo.GetByAddr(ctx, svr.GetAddr())

			if tt.serverExists {
				require.ErrorIs(t, probeErr, probing.ErrOutOfRetries)
				require.NoError(t, getErr)
				assert.Equal(t, tt.wantStatus, maybeUpdatedServer.GetDiscoveryStatus())
			} else {
				require.ErrorIs(t, probeErr, servers.ErrServerNotFound)
				require.ErrorIs(t, getErr, servers.ErrServerNotFound)
			}
		})
	}
}

func TestService_ProbePort_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"fresh server is updated",
			ds.New,
			ds.Details | ds.Info | ds.Port,
		},
		{
			"server is updated",
			ds.Details | ds.Master,
			ds.Details | ds.Master | ds.Info | ds.Port,
		},
		{
			"retried server is updated",
			ds.Master | ds.Details | ds.PortRetry,
			ds.Master | ds.Details | ds.Info | ds.Port,
		},
		{
			"dead server is updated",
			ds.NoPort | ds.NoDetails,
			ds.Details | ds.Info | ds.Port,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithPortSuggestions([]int{1, 4}),
			)

			responses := make(chan []byte)
			go func() {
				responses <- []byte(
					"\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10880" +
						"\\password\\false\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\153" +
						"\\timespecial\\0\\obj_Neutralize_All_Enemies\\1\\obj_Rescue_All_Hostages\\1" +
						"\\obj_Rescue_Rosenstein\\1\\obj_Rescue_Fillinger\\1\\obj_Rescue_Victims\\1" +
						"\\obj_Neutralize_Alice\\1\\tocreports\\8/13\\weaponssecured\\4/6\\queryid\\1\\final\\",
				)
			}()
			udp, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-4), svrAddr.Port-4)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			repo.AddOrUpdate(ctx, svr) // nolint: errcheck

			target := probes.New(svr.GetAddr(), svrAddr.Port, probes.GoalPort)

			probeErr := service.Probe(ctx, target)
			require.NoError(t, probeErr)

			queueSize, _ := queue.Count(ctx)
			assert.Equal(t, 0, queueSize)

			updatedSvr, _ := repo.GetByAddr(ctx, svr.GetAddr())
			assert.Equal(t, tt.wantStatus, updatedSvr.GetDiscoveryStatus())

			assert.Equal(t, svrAddr.Port, updatedSvr.GetQueryPort())

			updatedInfo := updatedSvr.GetInfo()
			assert.Equal(t, "-==MYT Co-op Svr==-", updatedInfo.Hostname)
			assert.Equal(t, "4/6", updatedInfo.WeaponsSecured)

			updatedDetails := updatedSvr.GetDetails()
			assert.Equal(t, "-==MYT Co-op Svr==-", updatedDetails.Info.Hostname)
			assert.Equal(t, "4/6", updatedDetails.Info.WeaponsSecured)
			assert.Len(t, updatedDetails.Objectives, 6)
			assert.Len(t, updatedDetails.Players, 0)
		})
	}
}

func TestService_ProbePort_RetryOnError(t *testing.T) {
	tests := []struct {
		name         string
		initStatus   ds.DiscoveryStatus
		serverExists bool
		respFactory  ResponseFunc
		wantStatus   ds.DiscoveryStatus
		wantErr      error
	}{
		{
			"positive case for existing server",
			ds.Details,
			true,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16" +
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
						"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.Details | ds.Info | ds.Port,
			nil,
		},
		{
			"good response for nonexistent server",
			ds.NoStatus,
			false,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16" +
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
						"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
		{
			"timeout for existing server",
			ds.Details | ds.Info,
			true,
			func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {},
			ds.Details | ds.Info | ds.PortRetry,
			probing.ErrProbeRetried,
		},
		{
			"timeout for nonexistent server",
			ds.NoStatus,
			false,
			func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
		{
			"no valid response for existing server",
			ds.Master | ds.Details | ds.Info,
			true,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.Master | ds.Details | ds.Info | ds.PortRetry,
			probing.ErrProbeRetried,
		},
		{
			"no valid response for nonexistent server",
			ds.NoStatus,
			false,
			func(ctx context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, bytes []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			ds.NoStatus,
			servers.ErrServerNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithMaxRetries(3),
				probing.WithPortSuggestions([]int{1, 4}),
			)

			udp, cancel := gs1.ServerFactory(tt.respFactory)
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-1), svrAddr.Port-1)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			if tt.serverExists {
				repo.AddOrUpdate(ctx, svr) // nolint: errcheck
			}

			target := probes.New(svr.GetAddr(), svrAddr.Port-4, probes.GoalPort)
			probeErr := service.Probe(ctx, target)

			queueSize, _ := queue.Count(ctx)

			updatedSvr, getErr := repo.GetByAddr(ctx, svr.GetAddr())
			updatedInfo := updatedSvr.GetInfo()

			if tt.wantErr != nil {
				require.ErrorIs(t, probeErr, tt.wantErr)
			} else {
				require.NoError(t, probeErr)
			}

			if !tt.serverExists { // nolint: gocritic
				require.ErrorIs(t, getErr, servers.ErrServerNotFound)
				assert.Equal(t, 0, queueSize)
			} else if tt.wantErr != nil {
				assert.Equal(t, svrAddr.Port-1, updatedSvr.GetQueryPort())
				assert.Equal(t, tt.wantStatus, updatedSvr.GetDiscoveryStatus())
				require.Equal(t, 1, queueSize)
				require.NoError(t, getErr)
				retry, _ := queue.PopAny(ctx)
				assert.Equal(t, 1, retry.GetRetries())
				assert.Equal(t, target.GetAddr(), retry.GetAddr())
				assert.Equal(t, target.GetPort(), retry.GetPort())
			} else {
				require.NoError(t, getErr)
				assert.Equal(t, 0, queueSize)
				assert.Equal(t, svrAddr.Port, updatedSvr.GetQueryPort())
				assert.Equal(t, "-==MYT Team Svr==-", updatedInfo.Hostname)
			}
		})
	}
}

func TestService_ProbePort_SelectedPortsAreTried(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		ok     bool
	}{
		{
			"join port",
			0,
			false,
		},
		{
			"+1 port",
			1,
			true,
		},
		{
			"+2 port",
			2,
			true,
		},
		{
			"+3 port",
			3,
			true,
		},
		{
			"+4 port",
			4,
			true,
		},
		{
			"+5 port",
			5,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithMaxRetries(3),
				probing.WithPortSuggestions([]int{1, 2, 3, 4}),
			)

			responses := make(chan []byte)
			go func() {
				responses <- []byte(
					"\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10880" +
						"\\password\\false\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\153" +
						"\\timespecial\\0\\obj_Neutralize_All_Enemies\\1\\obj_Rescue_All_Hostages\\1" +
						"\\obj_Rescue_Rosenstein\\1\\obj_Rescue_Fillinger\\1\\obj_Rescue_Victims\\1" +
						"\\obj_Neutralize_Alice\\1\\tocreports\\8/13\\weaponssecured\\4/6\\queryid\\1\\final\\",
				)
			}()
			udp, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-tt.offset), 12345)
			require.NoError(t, err)
			repo.AddOrUpdate(ctx, svr) // nolint: errcheck

			target := probes.New(svr.GetAddr(), 12345, probes.GoalPort)
			probeErr := service.Probe(ctx, target)

			updatedSvr, getErr := repo.GetByAddr(ctx, svr.GetAddr())
			require.NoError(t, getErr)

			if tt.ok {
				require.NoError(t, probeErr)
				updatedInfo := updatedSvr.GetInfo()
				assert.Equal(t, svrAddr.Port, updatedSvr.GetQueryPort())
				assert.Equal(t, "-==MYT Co-op Svr==-", updatedInfo.Hostname)
				assert.Equal(t, ds.Info|ds.Details|ds.Port, updatedSvr.GetDiscoveryStatus())

				_, popErr := queue.PopAny(ctx)
				assert.ErrorIs(t, popErr, probes.ErrQueueIsEmpty)
			} else {
				require.ErrorIs(t, probeErr, probing.ErrProbeRetried)
				assert.Equal(t, 12345, updatedSvr.GetQueryPort())
				assert.Equal(t, ds.PortRetry, updatedSvr.GetDiscoveryStatus())

				retry, _ := queue.PopAny(ctx)
				assert.Equal(t, 1, retry.GetRetries())
				assert.Equal(t, target.GetAddr(), retry.GetAddr())
				assert.Equal(t, target.GetPort(), retry.GetPort())
			}
		})
	}
}

func TestService_ProbePort_MultiplePortsAreProbed(t *testing.T) {
	udp1, cancel1 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
					"\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\10480\\password\\0" +
					"\\gamever\\1.1\\final\\\\queryid\\1.1",
			)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
		},
	)
	udpAddr1 := udp1.LocalAddr()
	defer cancel1()

	udp2, cancel2 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
					"\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\10480\\password\\false" +
					"\\gamever\\1.1\\round\\1\\numrounds\\5\\timeleft\\0\\timespecial\\0\\swatscore\\0" +
					"\\suspectsscore\\0\\swatwon\\1\\suspectswon\\1\\queryid\\1\\final\\",
			)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
		},
	)
	udpAddr2 := udp2.LocalAddr()
	defer cancel2()

	// same response as udp1
	udp3, cancel3 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
					"\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\10480\\password\\0" +
					"\\gamever\\1.1\\final\\\\queryid\\1.1",
			)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
		},
	)
	udpAddr3 := udp3.LocalAddr()
	defer cancel3()

	ctx := context.TODO()
	queue, repo, service := makeService(
		probing.WithProbeTimeout(time.Millisecond*10),
		probing.WithMaxRetries(3),
		probing.WithPortSuggestions(
			[]int{0, 2, udpAddr2.Port - udpAddr1.Port, udpAddr3.Port - udpAddr1.Port},
		),
	)

	svr, err := servers.NewFromAddr(addr.NewForTesting(udpAddr1.IP, udpAddr1.Port), 12345)
	require.NoError(t, err)
	repo.AddOrUpdate(ctx, svr) // nolint: errcheck

	target := probes.New(svr.GetAddr(), 12345, probes.GoalPort)
	probeErr := service.Probe(ctx, target)

	updatedSvr, getErr := repo.GetByAddr(ctx, svr.GetAddr())
	require.NoError(t, getErr)

	require.NoError(t, probeErr)
	updatedInfo := updatedSvr.GetInfo()
	assert.Equal(t, udpAddr2.Port, updatedSvr.GetQueryPort())
	assert.Equal(t, "-==MYT Team Svr==-", updatedInfo.Hostname)
	assert.Equal(t, 1, updatedInfo.SwatWon)
	assert.Equal(t, ds.Info|ds.Details|ds.Port, updatedSvr.GetDiscoveryStatus())

	_, popErr := queue.PopAny(ctx)
	assert.ErrorIs(t, popErr, probes.ErrQueueIsEmpty)
}

func TestService_ProbePort_OutOfRetries(t *testing.T) {
	tests := []struct {
		name         string
		initStatus   ds.DiscoveryStatus
		serverExists bool
		wantStatus   ds.DiscoveryStatus
	}{
		{
			"nonexistent server is skipped",
			ds.NoStatus,
			false,
			ds.NoStatus,
		},
		{
			"existing server is updated",
			ds.Details | ds.DetailsRetry | ds.Info | ds.PortRetry,
			true,
			ds.Details | ds.DetailsRetry | ds.Info | ds.NoPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			queue, repo, service := makeService(
				probing.WithProbeTimeout(time.Millisecond*10),
				probing.WithMaxRetries(1),
				probing.WithPortSuggestions([]int{1, 4}),
			)

			udp, cancel := gs1.ServerFactory(func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {})
			defer cancel()

			svrAddr := udp.LocalAddr()
			svr, err := servers.NewFromAddr(addr.NewForTesting(svrAddr.IP, svrAddr.Port-4), svrAddr.Port-4)
			require.NoError(t, err)

			if tt.initStatus.HasStatus() {
				svr.UpdateDiscoveryStatus(tt.initStatus)
			}

			if tt.serverExists {
				repo.AddOrUpdate(ctx, svr) // nolint: errcheck
			}

			target := probes.New(svr.GetAddr(), svrAddr.Port, probes.GoalPort)
			// pre-increment retry count
			_, incremented := target.IncRetries(1)
			require.True(t, incremented)

			probeErr := service.Probe(ctx, target)

			queueSize, _ := queue.Count(ctx)
			require.Equal(t, 0, queueSize)

			maybeUpdatedServer, getErr := repo.GetByAddr(ctx, svr.GetAddr())

			if tt.serverExists {
				require.ErrorIs(t, probeErr, probing.ErrOutOfRetries)
				require.NoError(t, getErr)
				assert.Equal(t, tt.wantStatus, maybeUpdatedServer.GetDiscoveryStatus())
				assert.Equal(t, svrAddr.Port-4, maybeUpdatedServer.GetQueryPort())
			} else {
				require.ErrorIs(t, probeErr, servers.ErrServerNotFound)
				require.ErrorIs(t, getErr, servers.ErrServerNotFound)
			}
		})
	}
}
