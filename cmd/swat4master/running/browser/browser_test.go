package browser_test

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
	"github.com/sergeii/swat4master/cmd/swat4master/running/browser"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/validation"
	gscrypt "github.com/sergeii/swat4master/pkg/gamespy/crypt"
)

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestBrowser_Run(t *testing.T) {
	var gameKey [6]byte
	var challenge [8]byte

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		BrowserServerLiveness: time.Hour,
		BrowserListenAddr:     "localhost:13382",
		BrowserClientTimeout:  time.Millisecond * 100,
	}

	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(browser.Run, ctx)
	runner.WaitReady()

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}))
	gs1.UpdateDiscoveryStatus(ds.Master)
	app.Servers.AddOrUpdate(ctx, gs1) // nolint: errcheck

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	gs2.UpdateDiscoveryStatus(ds.Details)
	app.Servers.AddOrUpdate(ctx, gs2) // nolint: errcheck

	copy(gameKey[:], "tG3j8c")
	copy(challenge[:], testutils.GenBrowserChallenge8())
	req := testutils.PackBrowserRequest(
		[]string{
			"hostname", "maxplayers", "gametype",
			"gamevariant", "mapname", "hostport",
			"password", "gamever", "statsenabled",
		},
		"gametype='VIP Escort'",
		[]byte{0x00, 0x00, 0x00, 0x00},
		func() []byte {
			return challenge[:]
		},
		testutils.CalcReqLength,
	)
	conn, _ := net.Dial("tcp", "127.0.0.1:13382")
	_, err := conn.Write(req)
	require.NoError(t, err)
	buf := make([]byte, 2048)
	cnt, err := conn.Read(buf)
	require.NoError(t, err)
	conn.Close()

	resp := gscrypt.Decrypt(gameKey, challenge, buf[:cnt])
	filteredServers := testutils.UnpackServerList(resp)
	assert.Len(t, filteredServers, 1)
	assert.Equal(t, "Swat4 Server", filteredServers[0]["hostname"])

	cancel()
	runner.WaitQuit()
}
