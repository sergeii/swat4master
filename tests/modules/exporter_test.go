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
	"github.com/sergeii/swat4master/cmd/swat4master/modules/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/observer"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reporter"
	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/instancefactory"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
	"github.com/sergeii/swat4master/tests/testapp"
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
		fx.Provide(testapp.ProvidePersistence),
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
	for range 2 {
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
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:     "localhost:11338",
				BrowserServerLiveness:  time.Second * 10,
				MetricObserverInterval: time.Millisecond,
			}
		}),
		exporter.Module,
		observer.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *observer.Observer) {}),
		fx.Populate(&repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	// Server is active but has no players
	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*5)),
		serverfactory.WithInfo(map[string]string{
			"gametype":   "VIP Escort",
			"numplayers": "0",
			"maxplayers": "16",
		}),
	)

	// Server is active and has players
	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*5)),
		serverfactory.WithInfo(map[string]string{
			"gametype":   "Barricaded Suspects",
			"numplayers": "12",
			"maxplayers": "16",
		}),
	)

	// Server is active and has players
	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*9)),
		serverfactory.WithInfo(map[string]string{
			"gametype":   "Smash And Grab",
			"numplayers": "1",
			"maxplayers": "10",
		}),
	)

	// Server is active and has players but has no Info status
	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.NoDetails),
		serverfactory.WithRefreshedAt(time.Now()),
		serverfactory.WithInfo(map[string]string{
			"gametype":   "VIP Escort",
			"numplayers": "14",
			"maxplayers": "16",
		}),
	)

	// Server is outdated and should not be included in the metrics
	serverfactory.Create(
		ctx,
		repo,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*11)),
		serverfactory.WithInfo(map[string]string{
			"gametype":   "Barricaded Suspects",
			"numplayers": "4",
			"maxplayers": "16",
		}),
	)

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
	countByStatus := make(map[string]int)
	for _, m := range mf["game_discovered_servers"].Metric {
		countByStatus[m.Label[0].GetValue()] = int(m.Gauge.GetValue())
	}
	assert.Equal(t, 3, countByStatus["master"])
	assert.Equal(t, 4, countByStatus["info"])
	assert.Equal(t, 2, countByStatus["details"])
	assert.Equal(t, 1, countByStatus["no_details"])
}

func TestExporter_ReposMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var serversRepo repositories.ServerRepository
	var instancesRepo repositories.InstanceRepository
	var probesRepo repositories.ProbeRepository

	app := fx.New(
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:     "localhost:11338",
				MetricObserverInterval: time.Millisecond,
			}
		}),
		exporter.Module,
		observer.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *observer.Observer) {}),
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
	for range 2 {
		inst := instancefactory.Build(instancefactory.WithRandomID(), instancefactory.WithRandomServerAddress())
		testutils.MustNoErr(instancesRepo.Add(ctx, inst))
	}

	probe1 := probe.New(svr1.Addr, svr1.QueryPort, probe.GoalDetails, 0)
	probe2 := probe.New(svr2.Addr, svr2.QueryPort, probe.GoalDetails, 0)
	testutils.MustNoErr(probesRepo.AddBetween(ctx, probe1, time.Now().Add(time.Hour), repositories.NC))
	testutils.MustNoErr(probesRepo.Add(ctx, probe2))

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
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				CleanRetention:         time.Millisecond * 10,
				CleanInterval:          time.Millisecond * 20,
				ExporterListenAddr:     "localhost:11338",
				MetricObserverInterval: time.Millisecond,
			}
		}),
		exporter.Module,
		observer.Module,
		cleaner.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *observer.Observer, *cleaner.Cleaner) {}),
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

	assert.Equal(t, 0, int(mf["cleaner_removals_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "instances", *mf["cleaner_removals_total"].Metric[0].Label[0].Value)

	assert.Equal(t, 2, int(mf["cleaner_removals_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, "servers", *mf["cleaner_removals_total"].Metric[1].Label[0].Value)
	assert.Equal(t, 0, int(mf["cleaner_errors_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "servers", *mf["cleaner_errors_total"].Metric[0].Label[0].Value)
}

func TestExporter_ProberMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var serverRepo repositories.ServerRepository
	var probeRepo repositories.ProbeRepository

	app := fx.New(
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ExporterListenAddr:     "localhost:11338",
				MetricObserverInterval: time.Millisecond,
				ProbeConcurrency:       2,
				ProbePollSchedule:      time.Millisecond,
				ProbeTimeout:           time.Millisecond * 250,
				DiscoveryRevivalPorts:  []int{0},
			}
		}),
		exporter.Module,
		observer.Module,
		prober.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Exporter, *observer.Observer, *prober.Prober) {}),
		fx.Populate(&serverRepo, &probeRepo),
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

	svr1, _ = serverRepo.Add(ctx, svr1, repositories.ServerOnConflictIgnore)
	svr2, _ = serverRepo.Add(ctx, svr2, repositories.ServerOnConflictIgnore)

	probe1 := probe.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probe.GoalDetails, 1)
	probe2 := probe.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probe.GoalPort, 1)
	probe3 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalDetails, 1)
	probe4 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalPort, 1)
	probe4.IncRetries()
	probe5 := probe.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probe.GoalDetails, 1)
	// will be launched immediately but will expire in 1s
	probeRepo.AddBetween(ctx, probe1, repositories.NC, time.Now().Add(time.Second)) // nolint: errcheck
	// will be launched no earlier than 100ms but will expire in 1s
	probeRepo.AddBetween( // nolint: errcheck
		ctx,
		probe2,
		time.Now().Add(time.Millisecond*100),
		time.Now().Add(time.Second),
	)
	probeRepo.AddBetween(ctx, probe3, time.Now().Add(time.Millisecond*300), repositories.NC) // nolint: errcheck
	probeRepo.AddBetween(ctx, probe4, time.Now().Add(time.Millisecond*300), repositories.NC) // nolint: errcheck
	// already expired
	probeRepo.AddBetween(ctx, probe5, repositories.NC, time.Now().Add(-time.Millisecond)) // nolint: errcheck

	<-time.After(time.Millisecond * 50)
	// 1 probe is picked and the worker is busy waiting for response
	mf := getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 0, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))
	assert.Nil(t, mf["discovery_probes_total"])

	<-time.After(time.Millisecond * 200)
	// port probe is picked, previous detail probe finished
	mf = getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 0, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
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
	assert.Equal(t, 0, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
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
	// one is retried, and the other one is considered failed
	mf = getMetrics(t)
	assert.Equal(t, 0, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
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
