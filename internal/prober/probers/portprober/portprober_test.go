package portprober_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/metrics"
	"github.com/sergeii/swat4master/internal/prober/probers/portprober"
	"github.com/sergeii/swat4master/internal/testutils/factories"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

func TestPortProber_Probe_OK(t *testing.T) {
	tests := []struct {
		name       string
		portOffset int
	}{
		{
			"+1",
			1,
		},
		{
			"+4",
			4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			proberOpts := portprober.Opts{Offsets: []int{1, 4}}
			prober := portprober.New(proberOpts, validate, clock, collector, &logger)

			responses := make(chan []byte)
			queryHandler, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()

			queryAddr := queryHandler.LocalAddr()
			svrAddr := addr.NewForTesting(queryAddr.IP, queryAddr.Port-tt.portOffset)

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

			result := ret.(portprober.Result) // nolint:forcetypeassert
			assert.Equal(t, queryAddr.Port, result.Port)

			det := result.Details
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
		})
	}
}

func TestPortProber_Probe_BestPort(t *testing.T) {
	tests := []struct {
		name               string
		portOffsetsFactory func(gmp int, vnp int, bdp int, amp int, gs1p int) []int
		wantedPortFactory  func(vnp int, bdp int, amp int, gs1p int) int
		wantedHostname     string
	}{
		{
			"vanilla response",
			func(gmp int, vnp int, bdp int, amp int, gs1p int) []int { // nolint:revive
				return []int{vnp - gmp, bdp - gmp}
			},
			func(vnp int, bdp int, amp int, gs1p int) int { // nolint:revive
				return vnp
			},
			"Vanilla Response",
		},
		{
			"admin mod response",
			func(gmp int, vnp int, bdp int, amp int, gs1p int) []int { // nolint:revive
				return []int{vnp - gmp, bdp - gmp, amp - gmp}
			},
			func(vnp int, bdp int, amp int, gs1p int) int { // nolint:revive
				return amp
			},
			"AM Response",
		},
		{
			"gs1 mod response",
			func(gmp int, vnp int, bdp int, amp int, gs1p int) []int {
				return []int{vnp - gmp, bdp - gmp, amp - gmp, gs1p - gmp}
			},
			func(vnp int, bdp int, amp int, gs1p int) int { // nolint:revive
				return gs1p
			},
			"GS1 response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			vanillaResponses := make(chan []byte)
			badResponses := make(chan []byte)
			amResponses := make(chan []byte)
			gs1Responses := make(chan []byte)

			vanillaHandler, cancelVanilla := gs1.PrepareGS1Server(vanillaResponses)
			defer cancelVanilla()

			badHandler, cancelBad := gs1.PrepareGS1Server(badResponses)
			defer cancelBad()

			amHandler, cancelAm := gs1.PrepareGS1Server(amResponses)
			defer cancelAm()

			gs1Handler, cancelGs1 := gs1.PrepareGS1Server(gs1Responses)
			defer cancelGs1()

			vanillaAddr := vanillaHandler.LocalAddr()
			vanillaPort := vanillaAddr.Port
			badPort := badHandler.LocalAddr().Port
			amPort := amHandler.LocalAddr().Port
			gs1Port := gs1Handler.LocalAddr().Port

			svrAddr := addr.NewForTesting(vanillaAddr.IP, vanillaPort-1)

			go func(ch chan []byte, gamePort int) {
				packet := []byte(
					fmt.Sprintf(
						"\\hostname\\Vanilla Response\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort"+
							"\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\%d\\password\\0"+
							"\\gamever\\1.1\\final\\\\queryid\\1.1",
						gamePort,
					),
				)
				ch <- packet
			}(vanillaResponses, svrAddr.Port)

			go func(ch chan []byte, gamePort int) {
				packet := []byte(
					fmt.Sprintf(
						"\\statusresponse\\0\\hostname\\AM Response\\numplayers\\0\\maxplayers\\16"+
							"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\%d"+
							"\\password\\0\\gamever\\1.1\\statsenabled\\0\\swatwon\\0\\suspectswon\\0\\round\\1"+
							"\\numrounds\\5\\queryid\\AMv1\\final\\\\eof\\",
						gamePort,
					),
				)
				ch <- packet
			}(amResponses, svrAddr.Port)

			go func(ch chan []byte, gamePort int) {
				packet := []byte(
					fmt.Sprintf(
						"\\hostname\\GS1 response\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort"+
							"\\gamevariant\\SWAT 4\\mapname\\Fairfax Residence\\hostport\\%d\\password\\false"+
							"\\gamever\\1.1\\round\\1\\numrounds\\5\\timeleft\\0\\timespecial\\0\\swatscore\\0"+
							"\\suspectsscore\\0\\swatwon\\1\\suspectswon\\1\\queryid\\1\\final\\",
						gamePort,
					),
				)
				ch <- packet
			}(gs1Responses, svrAddr.Port)

			portOffsets := tt.portOffsetsFactory(svrAddr.Port, vanillaPort, badPort, amPort, gs1Port)
			proberOpts := portprober.Opts{Offsets: portOffsets}
			prober := portprober.New(proberOpts, validate, clock, collector, &logger)

			ret, err := prober.Probe(ctx, svrAddr, svrAddr.Port, time.Millisecond*100)
			require.NoError(t, err)

			result := ret.(portprober.Result) // nolint:forcetypeassert
			assert.Equal(t, tt.wantedHostname, result.Details.Info.Hostname)

			wantedPort := tt.wantedPortFactory(vanillaPort, badPort, amPort, gs1Port)
			assert.Equal(t, wantedPort, result.Port)
		})
	}
}

func TestPortProber_Probe_Fail(t *testing.T) {
	tests := []struct {
		name        string
		respFactory func(int) []byte
		wantErr     error
	}{
		{
			"timeout",
			func(_ int) []byte { return nil },
			portprober.ErrPortDiscoveryFailed,
		},
		{
			"no valid response - no queryid",
			func(gamePort int) []byte {
				return []byte(
					fmt.Sprintf(
						"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype\\VIP Escort"+
							"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\%d"+
							"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0"+
							"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\",
						gamePort,
					),
				)
			},
			portprober.ErrPortDiscoveryFailed,
		},
		{
			"no valid response - hostport does not match the game port",
			func(gamePort int) []byte {
				return []byte(
					fmt.Sprintf(
						"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16\\gametype"+
							"\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Meat Barn Restaurant"+
							"\\hostport\\%d\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
						gamePort+1,
					),
				)
			},
			portprober.ErrPortDiscoveryFailed,
		},
		{
			"no valid response - invalid structure",
			func(gamePort int) []byte {
				return []byte(
					fmt.Sprintf(
						"\\hostname\\-==MYT Team Svr==-\\numplayers\\not-an-integer\\maxplayers\\16\\gametype\\VIP Escort"+
							"\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store\\hostport\\%d"+
							"\\password\\false\\gamever\\1.1\\round\\4\\numrounds\\5\\timeleft\\1\\timespecial\\0"+
							"\\swatscore\\0\\suspectsscore\\0\\swatwon\\3\\suspectswon\\0\\final\\\\queryid\\1.1",
						gamePort,
					),
				)
			},
			portprober.ErrParseFailed,
		},
		{
			"no valid response - validation",
			func(gamePort int) []byte {
				return []byte(
					fmt.Sprintf(
						"\\hostname\\-==MYT Team Svr==-\\numplayers\\-1\\maxplayers\\16"+
							"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store"+
							"\\hostport\\%d\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
						gamePort,
					),
				)
			},
			portprober.ErrValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			proberOpts := portprober.Opts{Offsets: []int{1, 4}}
			prober := portprober.New(proberOpts, validate, clock, collector, &logger)

			responses := make(chan []byte)
			queryHandler, cancel := gs1.PrepareGS1Server(responses)
			defer cancel()

			queryAddr := queryHandler.LocalAddr()
			svrAddr := addr.NewForTesting(queryAddr.IP, queryAddr.Port-1)

			go func(ch chan []byte, port int, factory func(int) []byte) {
				packet := factory(port)
				ch <- packet
			}(responses, svrAddr.Port, tt.respFactory)

			_, probeErr := prober.Probe(ctx, svrAddr, queryAddr.Port, time.Millisecond*100)
			require.ErrorIs(t, probeErr, tt.wantErr)
		})
	}
}

func TestPortProber_HandleRetry_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"no status - PortRetry is added",
			ds.NoStatus,
			ds.PortRetry,
		},
		{
			"have just NoPort - PortRetry is added",
			ds.Info | ds.Master | ds.NoPort,
			ds.Info | ds.Master | ds.NoPort | ds.PortRetry,
		},
		{
			"have no NoPort",
			ds.Info | ds.Details,
			ds.Info | ds.Details | ds.PortRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := portprober.New(portprober.Opts{}, validate, clock, collector, &logger)

			svr := factories.BuildServer(factories.WithDiscoveryStatus(tt.initStatus))

			updatedSvr := prober.HandleRetry(svr)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)
		})
	}
}

func TestPortProber_HandleFailure_OK(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		wantStatus ds.DiscoveryStatus
	}{
		{
			"no status - NoPort is added",
			ds.NoStatus,
			ds.NoPort,
		},
		{
			"have just NoPort - NoPort is retained",
			ds.NoPort,
			ds.NoPort,
		},
		{
			"have multiple statuses including PortRetry - PortRetry is removed and NoPort is added",
			ds.Details | ds.Info | ds.PortRetry,
			ds.Details | ds.Info | ds.NoPort,
		},
		{
			"have multiple statuses excluding PortRetry - NoPort is added",
			ds.Details | ds.Info | ds.Master,
			ds.Details | ds.Info | ds.Master | ds.NoPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			clock := clockwork.NewFakeClock()
			validate := validation.MustNew()
			collector := metrics.New()

			prober := portprober.New(portprober.Opts{}, validate, clock, collector, &logger)

			svr := factories.BuildServer(factories.WithDiscoveryStatus(tt.initStatus))

			updatedSvr := prober.HandleFailure(svr)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)
		})
	}
}
