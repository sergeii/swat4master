package browser_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/browser"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/testutils"
	gscrypt "github.com/sergeii/swat4master/pkg/gamespy/crypt"
)

func TestBrowser_Run(t *testing.T) {
	var gameKey [6]byte
	var challenge [8]byte

	var repo servers.Repository

	clockMock := clock.NewMock()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				BrowserServerLiveness: time.Hour,
				BrowserListenAddr:     "localhost:13382",
				BrowserClientTimeout:  time.Millisecond * 100,
			}
		}),
		fx.Decorate(func() clock.Clock { return clockMock }),
		browser.Module,
		fx.NopLogger,
		fx.Invoke(func(*browser.Browser) {}),
		fx.Populate(&repo),
	)
	app.Start(context.TODO()) // nolint: errcheck
	defer func() {
		app.Stop(context.TODO()) // nolint: errcheck
	}()

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10480, 10481)
	gs1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}), clockMock.Now())
	gs1.UpdateDiscoveryStatus(ds.Master)
	repo.Add(ctx, gs1, servers.OnConflictIgnore) // nolint: errcheck

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}), clockMock.Now())
	gs2.UpdateDiscoveryStatus(ds.Details)
	repo.Add(ctx, gs2, servers.OnConflictIgnore) // nolint: errcheck

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
}
