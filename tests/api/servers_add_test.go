package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/probe"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type serverAddReqSchema struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type serverAddRespSchema struct {
	Address        string `json:"address"`
	IP             string `json:"ip"`
	Port           int    `json:"port"`
	Hostname       string `json:"hostname"`
	HostnamePlain  string `json:"hostname_plain"`
	HostnameHTML   string `json:"hostname_html"`
	Passworded     bool   `json:"passworded"`
	GameName       string `json:"gamename"`
	GameVer        string `json:"gamever"`
	GameType       string `json:"gametype"`
	GameTypeSlug   string `json:"gametype_slug"`
	MapName        string `json:"mapname"`
	MapNameSlug    string `json:"mapname_slug"`
	PlayerNum      int    `json:"player_num"`
	PlayerMax      int    `json:"player_max"`
	RoundNum       int    `json:"round_num"`
	RoundMax       int    `json:"round_max"`
	TimeLeft       int    `json:"time_round"`
	TimeSpecial    int    `json:"time_special"`
	SwatScore      int    `json:"score_swat"`
	SuspectsScore  int    `json:"score_sus"`
	SwatWon        int    `json:"vict_swat"`
	SuspectsWon    int    `json:"vict_sus"`
	BombsDefused   int    `json:"bombs_defused"`
	BombsTotal     int    `json:"bombs_total"`
	TocReports     string `json:"coop_reports"`
	WeaponsSecured string `json:"coop_weapons"`
}

type serverAddErrorSchema struct {
	Error string `json:"error"`
}

func TestAPI_AddServer_SubmitNew(t *testing.T) {
	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	payload, _ := json.Marshal(serverAddReqSchema{ // nolint: errchkjson
		IP:   "1.1.1.1",
		Port: 10480,
	})
	resp := testutils.DoTestRequest(
		ts, http.MethodPost, "/api/servers", bytes.NewReader(payload),
		testutils.MustHaveNoBody(),
	)
	assert.Equal(t, 202, resp.StatusCode)

	svrCount, _ := repos.Servers.Count(ctx)
	require.Equal(t, 1, svrCount)

	addedSvr, err := repos.Servers.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, 10481, addedSvr.QueryPort)
	assert.Equal(t, ds.PortRetry, addedSvr.DiscoveryStatus)

	prbCount, _ := repos.Probes.Count(ctx)
	assert.Equal(t, 1, prbCount)
	addedPrb, err := repos.Probes.Peek(ctx)
	require.NoError(t, err)
	assert.Equal(t, probe.GoalPort, addedPrb.Goal)
	assert.Equal(t, "1.1.1.1:10480", addedPrb.Addr.String())
	assert.Equal(t, 10480, addedPrb.Port)
}

func TestAPI_AddServer_SubmitExisting(t *testing.T) {
	tests := []struct {
		name       string
		initStatus ds.DiscoveryStatus
		queued     bool
		wantStatus ds.DiscoveryStatus
		wantCode   int
	}{
		{
			"server discovery is already pending",
			ds.PortRetry,
			false,
			ds.PortRetry,
			202,
		},
		{
			"server has no details but discovery is in progress",
			ds.DetailsRetry,
			false,
			ds.DetailsRetry,
			202,
		},
		{
			"server has both details and port discovery in progress",
			ds.DetailsRetry | ds.PortRetry,
			false,
			ds.DetailsRetry | ds.PortRetry,
			202,
		},
		{
			"server has no port",
			ds.NoPort,
			false,
			ds.NoPort,
			410,
		},
		{
			"server is reporting to master but has no port",
			ds.Master | ds.Info | ds.NoPort,
			false,
			ds.Master | ds.Info | ds.NoPort,
			410,
		},
		{
			"server has both no details and no port",
			ds.NoDetails | ds.NoPort,
			false,
			ds.NoDetails | ds.NoPort,
			410,
		},
		{
			"server is new",
			ds.New,
			true,
			ds.PortRetry,
			202,
		},
		{
			"server has no details",
			ds.NoDetails,
			true,
			ds.NoDetails | ds.PortRetry,
			202,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
			defer cancel()

			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithQueryPort(10484),
				serverfactory.WithDiscoveryStatus(tt.initStatus),
			)

			payload, _ := json.Marshal(serverAddReqSchema{
				IP:   "1.1.1.1",
				Port: 10480,
			})
			resp := testutils.DoTestRequest(
				ts, http.MethodPost, "/api/servers", bytes.NewReader(payload),
				testutils.MustHaveNoBody(),
			)
			assert.Equal(t, tt.wantCode, resp.StatusCode)

			updatedSvr, err := repos.Servers.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
			require.NoError(t, err)
			assert.Equal(t, "1.1.1.1", updatedSvr.Addr.GetDottedIP())
			assert.Equal(t, 10480, updatedSvr.Addr.Port)
			assert.Equal(t, 10484, updatedSvr.QueryPort)
			assert.Equal(t, tt.wantStatus, updatedSvr.DiscoveryStatus)

			prbCount, err := repos.Probes.Count(ctx)
			require.NoError(t, err)
			if tt.queued {
				assert.Equal(t, 1, prbCount)
				addedPrb, err := repos.Probes.Peek(ctx)
				require.NoError(t, err)
				assert.Equal(t, probe.GoalPort, addedPrb.Goal)
				assert.Equal(t, "1.1.1.1:10480", addedPrb.Addr.String())
				assert.Equal(t, 10480, addedPrb.Port)
			} else {
				assert.Equal(t, 0, prbCount)
			}
		})
	}
}

func TestAPI_AddServer_AlreadyDiscovered(t *testing.T) {
	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	fields := map[string]string{
		"hostname":      `[C=FFFFFF]Swat4[\c] [b]Server`,
		"hostport":      "10480",
		"mapname":       "A-Bomb Nightclub",
		"gamever":       "1.1",
		"gamevariant":   "SWAT 4",
		"gametype":      "VIP Escort",
		"password":      "0",
		"numplayers":    "14",
		"maxplayers":    "16",
		"round":         "4",
		"numrounds":     "5",
		"timeleft":      "877",
		"swatscore":     "98",
		"suspectsscore": "76",
		"swatwon":       "1",
		"suspectswon":   "2",
	}

	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10484),
		serverfactory.WithDiscoveryStatus(ds.Details),
		serverfactory.WithInfo(fields),
	)

	payload, _ := json.Marshal(serverAddReqSchema{ // nolint: errchkjson
		IP:   "1.1.1.1",
		Port: 10480,
	})
	obj := serverAddRespSchema{}
	resp := testutils.DoTestRequest(
		ts, http.MethodPost, "/api/servers", bytes.NewReader(payload),
		testutils.MustBindJSON(&obj),
	)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, `[C=FFFFFF]Swat4[\c] [b]Server`, obj.Hostname)
	assert.Equal(t, "Swat4 Server", obj.HostnamePlain)
	assert.Equal(t, `<span style="color:#FFFFFF;">Swat4</span> Server`, obj.HostnameHTML)
	assert.Equal(t, "1.1.1.1:10480", obj.Address)
	assert.Equal(t, "1.1.1.1", obj.IP)
	assert.Equal(t, 10480, obj.Port)
	assert.Equal(t, false, obj.Passworded)
	assert.Equal(t, "SWAT 4", obj.GameName)
	assert.Equal(t, "VIP Escort", obj.GameType)
	assert.Equal(t, "vip-escort", obj.GameTypeSlug)
	assert.Equal(t, "1.1", obj.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", obj.MapName)
	assert.Equal(t, "a-bomb-nightclub", obj.MapNameSlug)

	// no probe is added
	prbCount, err := repos.Probes.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, prbCount)

	// server status is not affected
	notUpdatedSvr, _ := repos.Servers.Get(ctx, addr.MustNewFromDotted("1.1.1.1", 10480))
	assert.Equal(t, ds.Details, notUpdatedSvr.DiscoveryStatus)
}

func TestAPI_AddServer_ValidateAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port int
		want bool
	}{
		{
			"positive case",
			"1.1.1.1",
			10480,
			true,
		},
		{
			"negative port",
			"1.1.1.1",
			-10480,
			false,
		},
		{
			"zero port",
			"1.1.1.1",
			0,
			false,
		},
		{
			"port out of range",
			"1.1.1.1",
			65536,
			false,
		},
		{
			"empty ip address",
			"",
			10480,
			false,
		},
		{
			"invalid ip address",
			"300.400.500.700",
			10480,
			false,
		},
		{
			"private ip address",
			"127.0.0.1",
			10480,
			false,
		},
		{
			"v6 ip address",
			"2001:db8:3c4d:15::1a2f:1a2b",
			10480,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
			defer cancel()

			payload, _ := json.Marshal(serverAddReqSchema{
				IP:   tt.ip,
				Port: tt.port,
			})
			if tt.want {
				resp := testutils.DoTestRequest(
					ts, http.MethodPost, "/api/servers", bytes.NewReader(payload),
					testutils.MustHaveNoBody(),
				)
				assert.Equal(t, 202, resp.StatusCode)
			} else {
				apiError := serverAddErrorSchema{}
				resp := testutils.DoTestRequest(
					ts, http.MethodPost, "/api/servers", bytes.NewReader(payload),
					testutils.MustBindJSON(&apiError),
				)
				assert.Equal(t, 400, resp.StatusCode)
			}

			svrCount, err := repos.Servers.Count(ctx)
			require.NoError(t, err)
			prbCount, err := repos.Probes.Count(ctx)
			require.NoError(t, err)

			if tt.want {
				assert.Equal(t, 1, svrCount)
				assert.Equal(t, 1, prbCount)
			} else {
				assert.Equal(t, 0, svrCount)
				assert.Equal(t, 0, prbCount)
			}
		})
	}
}
