package modules_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/cleaner"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/collector"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reporter"
	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	ps "github.com/sergeii/swat4master/internal/services/probe"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

func sendUDP(address string, req []byte) {
	conn, _ := net.Dial("udp", address)
	defer conn.Close() // nolint: errcheck
	conn.Write(req)    // nolint: errcheck
}

func getMetrics(t *testing.T) map[string]*dto.MetricFamily {
	resp, err := http.Get("http://localhost:11338/metrics")
	require.NoError(t, err)
	defer resp.Body.Close() // nolint: errcheck
	assert.Equal(t, 200, resp.StatusCode)
	parser := expfmt.TextParser{}
	mf, err := parser.TextToMetricFamilies(resp.Body)
	require.NoError(t, err)
	return mf
}

func TestExporter_MasterMetrics(t *testing.T) {
	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:   "localhost:11338",
				ReporterListenAddr:   "localhost:33811",
				ReporterBufferSize:   1024,
				BrowserListenAddr:    "localhost:13381",
				BrowserClientTimeout: time.Millisecond * 100,
			}
		}),
		exporter.Module,
		reporter.Module,
		browser.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *browser.Browser, *reporter.Reporter) {}),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// give the reporter some time to start
	<-time.After(time.Millisecond * 50)

	// valid available request
	sendUDP("127.0.0.1:33811", []byte{0x09})

	// invalid keepalive request (no prior heartbeat)
	for i := 0; i < 2; i++ {
		sendUDP("127.0.0.1:33811", []byte{0x08, 0xde, 0xad, 0xbe, 0xef})
	}

	// valid server browser request
	req := testutils.PackBrowserRequest(
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		"gametype='VIP Escort'",
		[]byte{0x00, 0x00, 0x00, 0x00},
		testutils.GenBrowserChallenge8,
		testutils.CalcReqLength,
	)
	testutils.SendTCP("127.0.0.1:13381", req)

	// invalid browser request (no fields)
	req = testutils.PackBrowserRequest(
		[]string{},
		"",
		[]byte{0x00, 0x00, 0x00, 0x00},
		testutils.GenBrowserChallenge8,
		testutils.CalcReqLength,
	)
	conn, _ := net.Dial("tcp", "127.0.0.1:13381")
	_, err := conn.Write(req)
	require.NoError(t, err)

	mf := getMetrics(t)

	assert.True(t, mf["go_goroutines"].Metric[0].Gauge.GetValue() > 0)

	assert.Equal(t, 11, int(mf["reporter_received_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 7, int(mf["reporter_sent_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["reporter_requests_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "available", *mf["reporter_requests_total"].Metric[0].Label[0].Value)
	assert.Equal(t, 2, int(mf["reporter_errors_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 180, int(mf["browser_received_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 133, int(mf["browser_sent_bytes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["browser_requests_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["browser_errors_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 0, int(mf["reporter_removals_total"].Metric[0].Counter.GetValue()))
}

func TestExporter_ServerMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var repo repositories.ServerRepository

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:    "localhost:11338",
				BrowserServerLiveness: time.Second * 10,
				CollectorInterval:     time.Millisecond,
			}
		}),
		exporter.Module,
		collector.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *collector.Collector) {}),
		fx.Populate(&repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}), time.Now())
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)

	svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
		"numplayers":  "12",
		"maxplayers":  "16",
	}), time.Now())
	svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)

	svr3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	svr3.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Awesome Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "Smash And Grab",
		"numplayers":  "1",
		"maxplayers":  "10",
	}), time.Now())
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)

	svr4 := server.MustNew(net.ParseIP("4.4.4.4"), 10480, 10481)
	svr4.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Other Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "14",
		"maxplayers":  "16",
	}), time.Now())
	svr4.UpdateDiscoveryStatus(ds.NoDetails)

	svr1, _ = repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore)
	svr2, _ = repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore)
	svr3, _ = repo.Add(ctx, svr3, repositories.ServerOnConflictIgnore)
	svr4, _ = repo.Add(ctx, svr4, repositories.ServerOnConflictIgnore)

	// give the collector some time to run
	<-time.After(time.Millisecond * 50)

	mf := getMetrics(t)

	assert.Len(t, mf["game_players"].Metric, 2)
	assert.Equal(t, 12, int(mf["game_players"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, "Barricaded Suspects", mf["game_players"].Metric[0].Label[0].GetValue())
	assert.Equal(t, 1, int(mf["game_players"].Metric[1].Gauge.GetValue()))
	assert.Equal(t, "Smash And Grab", mf["game_players"].Metric[1].Label[0].GetValue())

	assert.Len(t, mf["game_active_servers"].Metric, 3)
	assert.Equal(t, 1, int(mf["game_active_servers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, "Barricaded Suspects", mf["game_active_servers"].Metric[0].Label[0].GetValue())
	assert.Equal(t, 1, int(mf["game_active_servers"].Metric[1].Gauge.GetValue()))
	assert.Equal(t, "Smash And Grab", mf["game_active_servers"].Metric[1].Label[0].GetValue())
	assert.Equal(t, 1, int(mf["game_active_servers"].Metric[2].Gauge.GetValue()))
	assert.Equal(t, "VIP Escort", mf["game_active_servers"].Metric[2].Label[0].GetValue())

	assert.Len(t, mf["game_played_servers"].Metric, 2)
	assert.Equal(t, 1, int(mf["game_played_servers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, "Barricaded Suspects", mf["game_played_servers"].Metric[0].Label[0].GetValue())
	assert.Equal(t, 1, int(mf["game_played_servers"].Metric[1].Gauge.GetValue()))
	assert.Equal(t, "Smash And Grab", mf["game_played_servers"].Metric[1].Label[0].GetValue())

	assert.Len(t, mf["game_discovered_servers"].Metric, 4)
	assert.Equal(t, 2, int(mf["game_discovered_servers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, "details", mf["game_discovered_servers"].Metric[0].Label[0].GetValue())
	assert.Equal(t, 3, int(mf["game_discovered_servers"].Metric[1].Gauge.GetValue()))
	assert.Equal(t, "info", mf["game_discovered_servers"].Metric[1].Label[0].GetValue())
	assert.Equal(t, 2, int(mf["game_discovered_servers"].Metric[2].Gauge.GetValue()))
	assert.Equal(t, "master", mf["game_discovered_servers"].Metric[2].Label[0].GetValue())
	assert.Equal(t, 1, int(mf["game_discovered_servers"].Metric[3].Gauge.GetValue()))
	assert.Equal(t, "no_details", mf["game_discovered_servers"].Metric[3].Label[0].GetValue())
}

func TestExporter_ReposMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var serversRepo repositories.ServerRepository
	var instancesRepo repositories.InstanceRepository
	var probesRepo repositories.ProbeRepository

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr: "localhost:11338",
				CollectorInterval:  time.Millisecond,
			}
		}),
		exporter.Module,
		collector.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *collector.Collector) {}),
		fx.Populate(&serversRepo, &instancesRepo, &probesRepo),
	)

	// servers
	svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr3 := server.MustNew(net.ParseIP("3.3.3.3"), 10480, 10481)
	serversRepo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck
	serversRepo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck
	serversRepo.Add(ctx, svr3, repositories.ServerOnConflictIgnore) // nolint: errcheck

	// instances
	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	ins2 := instance.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	instancesRepo.Add(ctx, ins1) // nolint: errcheck
	instancesRepo.Add(ctx, ins2) // nolint: errcheck

	probe1 := probe.New(svr1.Addr, svr1.QueryPort, probe.GoalDetails)
	probe2 := probe.New(svr2.Addr, svr2.QueryPort, probe.GoalDetails)
	probesRepo.AddBetween(ctx, probe1, time.Now().Add(time.Hour), repositories.NC) // nolint: errcheck
	probesRepo.Add(ctx, probe2)                                                    // nolint: errcheck

	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// give the collector some time to run
	<-time.After(time.Millisecond * 50)

	mf := getMetrics(t)

	assert.Equal(t, 3, int(mf["repo_servers_size"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["repo_instances_size"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["repo_probes_size"].Metric[0].Gauge.GetValue()))
}

func TestExporter_CleanerMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var repo repositories.ServerRepository

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				CleanRetention:     time.Millisecond * 10,
				CleanInterval:      time.Millisecond * 20,
				ExporterListenAddr: "localhost:11338",
				CollectorInterval:  time.Millisecond,
			}
		}),
		exporter.Module,
		collector.Module,
		cleaner.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *collector.Collector, *cleaner.Cleaner) {}),
		fx.Populate(&repo),
	)

	svr1 := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore) // nolint: errcheck
	svr2 := server.MustNew(net.ParseIP("2.2.2.2"), 10480, 10481)
	repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore) // nolint: errcheck

	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// give the cleaner some time to run
	<-time.After(time.Millisecond * 100)

	resp, err := http.Get("http://localhost:11338/metrics")
	require.NoError(t, err)
	defer resp.Body.Close() // nolint: errcheck
	assert.Equal(t, 200, resp.StatusCode)
	parser := expfmt.TextParser{}
	mf, _ := parser.TextToMetricFamilies(resp.Body)

	assert.Equal(t, 2, int(mf["cleaner_removals_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 0, int(mf["cleaner_errors_total"].Metric[0].Counter.GetValue()))
}

func TestExporter_ProberMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var repo repositories.ServerRepository
	var probeService *ps.Service

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:    "localhost:11338",
				CollectorInterval:     time.Millisecond,
				ProbeConcurrency:      2,
				ProbePollSchedule:     time.Millisecond,
				ProbeRetries:          1,
				ProbeTimeout:          time.Millisecond * 250,
				DiscoveryRevivalPorts: []int{0},
			}
		}),
		exporter.Module,
		collector.Module,
		prober.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *collector.Collector, *prober.Prober) {}),
		fx.Populate(&repo, &probeService),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()
	runtime.Gosched()

	udp1, cancelSvr1 := gs1.ServerFactory(
		func(_ context.Context, conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
			udpAddr, _ := conn.LocalAddr().(*net.UDPAddr)
			packet := []byte(
				fmt.Sprintf(
					"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16"+
						"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store"+
						"\\hostport\\%d\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
					udpAddr.Port,
				),
			)
			<-time.After(time.Millisecond * 200)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
		},
	)
	addr1 := udp1.LocalAddr()
	defer cancelSvr1()

	udp2, cancelSvr2 := gs1.ServerFactory(
		func(_ context.Context, _ *net.UDPConn, _ *net.UDPAddr, _ []byte) {},
	)
	addr2 := udp2.LocalAddr()
	defer cancelSvr2()

	svr1, err := server.NewFromAddr(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port)
	require.NoError(t, err)
	svr1.UpdateDiscoveryStatus(ds.Port)

	svr2, err := server.NewFromAddr(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port)
	require.NoError(t, err)
	svr2.UpdateDiscoveryStatus(ds.Port)

	svr1, _ = repo.Add(ctx, svr1, repositories.ServerOnConflictIgnore)
	svr2, _ = repo.Add(ctx, svr2, repositories.ServerOnConflictIgnore)

	probe1 := probe.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probe.GoalDetails)
	probe2 := probe.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probe.GoalPort)
	probe3 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalDetails)
	probe4 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalPort)
	probe4.IncRetries(2)
	probe5 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalDetails)
	// will be launched immediately but will expire in 1s
	probeService.AddBefore(ctx, probe1, time.Now().Add(time.Second)) // nolint: errcheck
	// will be launched no earlier than 100ms but will expire in 1s
	probeService.AddBetween( // nolint: errcheck
		ctx,
		probe2,
		time.Now().Add(time.Millisecond*100),
		time.Now().Add(time.Second),
	)
	probeService.AddAfter(ctx, probe3, time.Now().Add(time.Millisecond*300)) // nolint: errcheck
	probeService.AddAfter(ctx, probe4, time.Now().Add(time.Millisecond*300)) // nolint: errcheck
	// already expired
	probeService.AddBefore(ctx, probe5, time.Now().Add(-time.Millisecond)) // nolint: errcheck

	<-time.After(time.Millisecond * 50)
	// 1 probe is picked and the worker is busy waiting for response
	mf := getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 5, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))
	assert.Nil(t, mf["discovery_probes_total"])

	<-time.After(time.Millisecond * 200)
	// port probe is picked, previous detail probe finished
	mf = getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 5, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 2, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 1, int(mf["discovery_probes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probes_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probes_total"].Metric, 1)

	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_success_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probe_success_total"].Metric, 1)

	assert.Nil(t, mf["discovery_probe_failures_total"])
	assert.Nil(t, mf["discovery_probe_errors_total"])

	<-time.After(time.Millisecond * 200)
	// details and port probes for unresponsive server are picked
	// previous probes are finished
	mf = getMetrics(t)
	assert.Equal(t, 2, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 0, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 5, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 4, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 1, int(mf["discovery_probes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probes_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probes_total"].Metric[0].Label[0].Value)
	assert.Equal(t, "port", *mf["discovery_probes_total"].Metric[1].Label[0].Value)

	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_success_total"].Metric[0].Label[0].Value)
	assert.Equal(t, "port", *mf["discovery_probe_success_total"].Metric[1].Label[0].Value)

	assert.Nil(t, mf["discovery_probe_failures_total"])
	assert.Nil(t, mf["discovery_probe_retries_total"])

	<-time.After(time.Millisecond * 200)
	// both probes failed due to timeout
	mf = getMetrics(t)
	assert.Equal(t, 0, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 6, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 4, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 2, int(mf["discovery_probes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 2, int(mf["discovery_probes_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[1].Counter.GetValue()))

	assert.Equal(t, 1, int(mf["discovery_probe_failures_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "port", *mf["discovery_probe_failures_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probe_failures_total"].Metric, 1)

	assert.Equal(t, 1, int(mf["discovery_probe_retries_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_retries_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probe_retries_total"].Metric, 1)
}
