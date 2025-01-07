package detailsprober_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/prober/probers/detailsprober"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

type ResponseFunc func(context.Context, *net.UDPConn, *net.UDPAddr, []byte)

func TestDetailsProber_Probe_OK(t *testing.T) {
	ctx := context.TODO()
	logger := zerolog.Nop()
	clock := clockwork.NewFakeClock()
	validate := validation.MustNew()
	collector := metrics.New()

	prober := detailsprober.New(validate, clock, collector, &logger)

	responses := make(chan []byte)
	queryHandler, cancel := gs1.PrepareGS1Server(responses)
	defer cancel()

	queryAddr := queryHandler.LocalAddr()
	svrAddr := addr.NewForTesting(queryAddr.IP, queryAddr.Port-1)

	go func(gamePort int) {
		responses <- []byte(
			fmt.Sprintf(
				"\\hostname\\-==MYT Co-op Svr==-\\numplayers\\0\\maxplayers\\5\\gametype\\CO-OP"+
					"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\%d"+
					"\\password\\false\\gamever\\1.1\\round\\1\\numrounds\\1\\timeleft\\153"+
					"\\timespecial\\0\\obj_Neutralize_All_Enemies\\1\\obj_Rescue_All_Hostages\\1"+
					"\\obj_Rescue_Rosenstein\\1\\obj_Rescue_Fillinger\\1\\obj_Rescue_Victims\\1"+
					"\\obj_Neutralize_Alice\\1\\tocreports\\8/13\\weaponssecured\\4/6\\queryid\\1\\final\\",
				gamePort,
			),
		)
	}(svrAddr.Port)

	ret, err := prober.Probe(ctx, svrAddr, queryAddr.Port, time.Millisecond*100)
	require.NoError(t, err)

	det := ret.(details.Details) // nolint:forcetypeassert
	assert.Equal(t, "-==MYT Co-op Svr==-", det.Info.Hostname)
	assert.Equal(t, 0, det.Info.NumPlayers)
	assert.Equal(t, 5, det.Info.MaxPlayers)
	assert.Equal(t, "CO-OP", det.Info.GameType)
	assert.Equal(t, "SWAT 4", det.Info.GameVariant)
	assert.Equal(t, "Qwik Fuel Convenience Store", det.Info.MapName)
	assert.Equal(t, "8/13", det.Info.TocReports)
	assert.Equal(t, "4/6", det.Info.WeaponsSecured)
	assert.Len(t, det.Objectives, 6)
	assert.Len(t, det.Players, 0)
}

func TestDetailsProber_Probe_Fail(t *testing.T) {
	tests := []struct {
		name        string
		respFactory ResponseFunc
		wantErr     error
	}{
		{
			"timeout",
			func(context.Context, *net.UDPConn, *net.UDPAddr, []byte) {},
			detailsprober.ErrQueryTimeout,
		},
		{
			"no valid response - no queryid",
			func(_ context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, _ []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			detailsprober.ErrQueryFailed,
		},
		{
			"no valid response - invalid structure",
			func(_ context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, _ []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\not-an-integer\\maxplayers\\16\\gametype\\VIP Escort" +
						"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\10480" +
						"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0" +
						"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			detailsprober.ErrParseFailed,
		},
		{
			"no valid response - validation",
			func(_ context.Context, conn *net.UDPConn, udpAddr *net.UDPAddr, _ []byte) {
				packet := []byte(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\-1\\maxplayers\\16" +
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
						"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
				)
				conn.WriteToUDP(packet, udpAddr) // nolint: errcheck
			},
			detailsprober.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := detailsprober.New(validate, clock, collector, &logger)

			queryHandler, cancel := gs1.ServerFactory(tt.respFactory)
			defer cancel()

			queryAddr := queryHandler.LocalAddr()
			svrAddr := addr.NewForTesting(queryAddr.IP, queryAddr.Port-1)

			_, probeErr := prober.Probe(ctx, svrAddr, queryAddr.Port, time.Millisecond*100)
			require.ErrorIs(t, probeErr, tt.wantErr)
		})
	}
}

func TestDetailsProber_HandleSuccess_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"no status",
			ds.NoStatus,
			ds.Info | ds.Details,
		},
		{
			"have just NoDetails",
			ds.NoDetails,
			ds.Info | ds.Details,
		},
		{
			"already have Info and Details",
			ds.Info | ds.Details | ds.Master,
			ds.Info | ds.Details | ds.Master,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := detailsprober.New(validate, clock, collector, &logger)

			svr := serverfactory.Build(serverfactory.WithDiscoveryStatus(tt.initStatus))
			params := testutils.GenExtraServerParams(map[string]string{"mapname": "A-Bomb Nightclub"})
			det := details.MustNewDetailsFromParams(params, nil, nil)

			updatedSvr := prober.HandleSuccess(det, svr)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)

			updatedSvr.RefreshedAt = clock.Now()
		})
	}
}

func TestDetailsProber_HandleRetry_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"no status",
			ds.NoStatus,
			ds.DetailsRetry,
		},
		{
			"have NoDetails",
			ds.Info | ds.Master | ds.NoDetails,
			ds.Info | ds.Master | ds.NoDetails | ds.DetailsRetry,
		},
		{
			"have no NoDetails",
			ds.Info | ds.Details,
			ds.Info | ds.Details | ds.DetailsRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := detailsprober.New(validate, clock, collector, &logger)

			svr := serverfactory.Build(serverfactory.WithDiscoveryStatus(tt.initStatus))

			updatedSvr := prober.HandleRetry(svr)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)
		})
	}
}

func TestDetailsProber_HandleFailure_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"no status",
			ds.NoStatus,
			ds.NoDetails,
		},
		{
			"have just NoDetails",
			ds.NoDetails,
			ds.NoDetails,
		},
		{
			"have multiple statuses but NoDetails is not among them",
			ds.Details | ds.Info | ds.DetailsRetry | ds.Port | ds.Master,
			ds.Master | ds.NoDetails,
		},
		{
			"have multiple statuses and NoDetails is among them",
			ds.Details | ds.Info | ds.Master | ds.NoDetails,
			ds.Master | ds.NoDetails,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := detailsprober.New(validate, clock, collector, &logger)

			svr := serverfactory.Build(serverfactory.WithDiscoveryStatus(tt.initStatus))

			updatedSvr := prober.HandleFailure(svr)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)
		})
	}
}
