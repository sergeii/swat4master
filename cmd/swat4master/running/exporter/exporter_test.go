package exporter_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/browser"
	"github.com/sergeii/swat4master/cmd/swat4master/running/cleaner"
	"github.com/sergeii/swat4master/cmd/swat4master/running/collector"
	"github.com/sergeii/swat4master/cmd/swat4master/running/exporter"
	"github.com/sergeii/swat4master/cmd/swat4master/running/prober"
	"github.com/sergeii/swat4master/cmd/swat4master/running/reporter"
	"github.com/sergeii/swat4master/internal/core/instances"
	"github.com/sergeii/swat4master/internal/core/probes"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/gs1"
)

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func send(address string, req []byte) {
	conn, _ := net.Dial("udp", address)
	defer conn.Close() // nolint: errcheck
	// valid available request
	conn.Write(req) // nolint: errcheck
}

func TestExporter_MasterMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		ExporterListenAddr:   "localhost:11338",
		ReporterListenAddr:   "localhost:33811",
		ReporterBufferSize:   1024,
		BrowserListenAddr:    "localhost:13381",
		BrowserClientTimeout: time.Millisecond * 100,
	}

	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(exporter.Run, ctx)
	runner.Add(reporter.Run, ctx)
	runner.Add(browser.Run, ctx)
	runner.WaitReady()

	// valid available request
	send("127.0.0.1:33811", []byte{0x09})

	// invalid keepalive request (no prior heartbeat)
	for i := 0; i < 2; i++ {
		send("127.0.0.1:33811", []byte{0x08, 0xde, 0xad, 0xbe, 0xef})
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
	buf := make([]byte, 1024)
	conn, _ := net.Dial("tcp", "127.0.0.1:13381")
	_, err := conn.Write(req)
	require.NoError(t, err)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	conn.Close()

	// invalid browser request (no fields)
	req = testutils.PackBrowserRequest(
		[]string{},
		"",
		[]byte{0x00, 0x00, 0x00, 0x00},
		testutils.GenBrowserChallenge8,
		testutils.CalcReqLength,
	)
	conn, _ = net.Dial("tcp", "127.0.0.1:13381")
	_, err = conn.Write(req)
	require.NoError(t, err)

	<-time.After(time.Millisecond * 5)
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

	cancel()
	runner.WaitQuit()
}

func TestExporter_ServerMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		ExporterListenAddr:    "localhost:11338",
		BrowserServerLiveness: time.Second * 10,
		CollectorInterval:     time.Millisecond,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(exporter.Run, ctx)
	runner.WaitReady()
	runner.Add(collector.Run, ctx)

	svr1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}))
	svr1.UpdateDiscoveryStatus(ds.Master | ds.Info)

	svr2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
		"numplayers":  "12",
		"maxplayers":  "16",
	}))
	svr2.UpdateDiscoveryStatus(ds.Details | ds.Info)

	svr3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	svr3.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Awesome Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4X",
		"gametype":    "Smash And Grab",
		"numplayers":  "1",
		"maxplayers":  "10",
	}))
	svr3.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)

	svr4, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	svr4.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Other Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"numplayers":  "14",
		"maxplayers":  "16",
	}))
	svr4.UpdateDiscoveryStatus(ds.NoDetails)

	app.Servers.AddOrUpdate(ctx, svr1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr2) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr3) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr4) // nolint: errcheck

	<-time.After(time.Millisecond * 5)
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

	cancel()
	runner.WaitQuit()
}

func TestExporter_ReposMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		ExporterListenAddr: "localhost:11338",
		CollectorInterval:  time.Millisecond,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(exporter.Run, ctx)
	runner.WaitReady()
	runner.Add(collector.Run, ctx)

	// servers
	svr1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	svr2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	svr3, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	app.Servers.AddOrUpdate(ctx, svr1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr2) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr3) // nolint: errcheck

	// instances
	ins1 := instances.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	ins2 := instances.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	app.Instances.Add(ctx, ins1) // nolint: errcheck
	app.Instances.Add(ctx, ins2) // nolint: errcheck

	probe1 := probes.New(svr1.GetAddr(), svr1.GetQueryPort(), probes.GoalDetails)
	probe2 := probes.New(svr2.GetAddr(), svr2.GetQueryPort(), probes.GoalDetails)
	app.Probes.AddBetween(ctx, probe1, time.Now().Add(time.Hour), probes.NC) // nolint: errcheck
	app.Probes.Add(ctx, probe2)                                              // nolint: errcheck

	<-time.After(time.Millisecond * 5)
	mf := getMetrics(t)

	assert.Equal(t, 3, int(mf["repo_servers_size"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["repo_instances_size"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 2, int(mf["repo_probes_size"].Metric[0].Gauge.GetValue()))

	cancel()
	runner.WaitQuit()
}

func TestExporter_CleanerMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		CleanRetention:     time.Millisecond * 10,
		CleanInterval:      time.Millisecond * 20,
		ExporterListenAddr: "localhost:11338",
		CollectorInterval:  time.Millisecond,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(exporter.Run, ctx)
	runner.WaitReady()
	runner.Add(collector.Run, ctx)
	runner.Add(cleaner.Run, ctx)

	svr1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	_ = app.Servers.AddOrUpdate(ctx, svr1)
	svr2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	_ = app.Servers.AddOrUpdate(ctx, svr2)

	<-time.After(time.Millisecond * 50)

	resp, err := http.Get("http://localhost:11338/metrics")
	require.NoError(t, err)
	defer resp.Body.Close() // nolint: errcheck
	assert.Equal(t, 200, resp.StatusCode)
	parser := expfmt.TextParser{}
	mf, _ := parser.TextToMetricFamilies(resp.Body)

	assert.Equal(t, 2, int(mf["cleaner_removals_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 0, int(mf["cleaner_errors_total"].Metric[0].Counter.GetValue()))

	cancel()
	runner.WaitQuit()
}

func TestExporter_ProberMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		ExporterListenAddr:    "localhost:11338",
		CollectorInterval:     time.Millisecond,
		ProbeConcurrency:      2,
		ProbePollSchedule:     time.Millisecond,
		ProbeRetries:          1,
		ProbeTimeout:          time.Millisecond * 50,
		DiscoveryRevivalPorts: []int{0},
	}
	app := application.Configure()

	udp1, cancelSvr1 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
			packet := []byte(
				"\\hostname\\-==MYT Team Svr==-\\numplayers\\0\\maxplayers\\16" +
					"\\gametype\\VIP Escort\\gamevariant\\SWAT 4\\mapname\\Qwik Fuel Convenience Store" +
					"\\hostport\\10480\\password\\0\\gamever\\1.1\\final\\\\queryid\\1.1",
			)
			<-time.After(time.Millisecond * 45)
			conn.WriteToUDP(packet, addr) // nolint: errcheck
		},
	)
	addr1 := udp1.LocalAddr()
	defer cancelSvr1()

	udp2, cancelSvr2 := gs1.ServerFactory(
		func(ctx context.Context, conn *net.UDPConn, addr *net.UDPAddr, req []byte) {},
	)
	addr2 := udp2.LocalAddr()
	defer cancelSvr2()

	svr1, err := servers.NewFromAddr(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port)
	require.NoError(t, err)
	svr1.UpdateDiscoveryStatus(ds.Port)

	svr2, err := servers.NewFromAddr(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port)
	require.NoError(t, err)
	svr2.UpdateDiscoveryStatus(ds.Port)

	app.Servers.AddOrUpdate(ctx, svr1) // nolint: errcheck
	app.Servers.AddOrUpdate(ctx, svr2) // nolint: errcheck

	probe1 := probes.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probes.GoalDetails)
	probe2 := probes.New(addr.NewForTesting(addr1.IP, addr1.Port), addr1.Port, probes.GoalPort)
	probe3 := probes.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probes.GoalDetails)
	probe4 := probes.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probes.GoalPort)
	probe4.IncRetries(2)
	probe5 := probes.New(addr.NewForTesting(addr2.IP, addr2.Port), addr2.Port, probes.GoalDetails)
	app.ProbeService.AddBefore(ctx, probe1, time.Now().Add(time.Millisecond*100)) // nolint: errcheck
	app.ProbeService.AddBetween(                                                  // nolint: errcheck
		ctx,
		probe2,
		time.Now().Add(time.Millisecond*50),
		time.Now().Add(time.Millisecond*500),
	)
	app.ProbeService.AddAfter(ctx, probe3, time.Now().Add(time.Millisecond*50)) // nolint: errcheck
	app.ProbeService.AddAfter(ctx, probe4, time.Now().Add(time.Millisecond*50)) // nolint: errcheck
	app.ProbeService.AddBefore(ctx, probe5, time.Now().Add(-time.Millisecond))  // nolint: errcheck

	runner := running.NewRunner(app, cfg)
	runner.Add(exporter.Run, ctx)
	runner.WaitReady()
	runner.Add(collector.Run, ctx)
	runner.Add(prober.Run, ctx)

	<-time.After(time.Millisecond * 5)
	mf := getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 5, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	<-time.After(time.Millisecond * 50)
	mf = getMetrics(t)
	assert.Equal(t, 2, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 0, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 5, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 3, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 1, int(mf["discovery_probes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probes_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probes_total"].Metric, 1)

	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_success_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probe_success_total"].Metric, 1)

	assert.Nil(t, mf["discovery_probe_failures_total"])
	assert.Nil(t, mf["discovery_probe_errors_total"])

	<-time.After(time.Millisecond * 55)
	mf = getMetrics(t)
	assert.Equal(t, 1, int(mf["discovery_busy_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_available_workers"].Metric[0].Gauge.GetValue()))
	assert.Equal(t, 6, int(mf["discovery_queue_produced_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 4, int(mf["discovery_queue_consumed_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_queue_expired_total"].Metric[0].Counter.GetValue()))

	assert.Equal(t, 2, int(mf["discovery_probes_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probes_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probes_total"].Metric[0].Label[0].Value)
	assert.Equal(t, "port", *mf["discovery_probes_total"].Metric[1].Label[0].Value)

	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, 1, int(mf["discovery_probe_success_total"].Metric[1].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_success_total"].Metric[0].Label[0].Value)
	assert.Equal(t, "port", *mf["discovery_probe_success_total"].Metric[1].Label[0].Value)

	assert.Nil(t, mf["discovery_probe_failures_total"])
	assert.Equal(t, 1, int(mf["discovery_probe_retries_total"].Metric[0].Counter.GetValue()))
	assert.Equal(t, "details", *mf["discovery_probe_retries_total"].Metric[0].Label[0].Value)
	assert.Len(t, mf["discovery_probe_retries_total"].Metric, 1)

	<-time.After(time.Millisecond * 55)
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

	cancel()
	runner.WaitQuit()
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
