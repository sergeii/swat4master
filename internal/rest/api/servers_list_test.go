package api_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/details"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/validation"
)

type serverListSchema struct {
	Address       string `json:"address"`
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Hostname      string `json:"hostname"`
	HostnamePlain string `json:"hostname_plain"`
	HostnameHTML  string `json:"hostname_html"`
	Passworded    bool   `json:"passworded"`
	GameName      string `json:"gamename"`
	GameVer       string `json:"gamever"`
	GameType      string `json:"gametype"`
	GameTypeSlug  string `json:"gametype_slug"`
	MapName       string `json:"mapname"`
	MapNameSlug   string `json:"mapname_slug"`
	PlayerNum     int    `json:"player_num"`
	PlayerMax     int    `json:"player_max"`
	RoundNum      int    `json:"round_num"`
	RoundMax      int    `json:"round_max"`
}

func TestMain(m *testing.M) {
	if err := validation.Register(); err != nil {
		panic(err)
	}
	m.Run()
}

func TestAPI_ListServers_OK(t *testing.T) {
	ctx := context.TODO()
	ts, app, cancel := testutils.PrepareTestServer(func(cfg *config.Config) {
		cfg.BrowserServerLiveness = time.Millisecond * 15
	})
	defer cancel()

	outdated, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	outdated.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
		"password":    "0",
		"numplayers":  "15",
		"maxplayers":  "16",
		"round":       "1",
		"numrounds":   "5",
	}))
	outdated.UpdateDiscoveryStatus(ds.Master)
	_ = app.Servers.AddOrUpdate(ctx, outdated)
	time.Sleep(time.Millisecond * 15)

	noStatus, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	noStatus.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Cool Swat4 Server",
		"hostport":    "10480",
		"mapname":     "Riverside Training Facility",
		"gamever":     "1.0",
		"gamevariant": "SWAT 4",
		"gametype":    "Barricaded Suspects",
	}))
	_ = app.Servers.AddOrUpdate(ctx, noStatus)
	time.Sleep(time.Millisecond * 1)

	delisted, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	delisted.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "COOP Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "CO-OP",
	}))
	delisted.UpdateDiscoveryStatus(ds.NoDetails)
	_ = app.Servers.AddOrUpdate(ctx, delisted)
	time.Sleep(time.Millisecond * 1)

	noInfo, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	delisted.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Awesome Server",
		"hostport":    "10580",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "CO-OP",
	}))
	noInfo.UpdateDiscoveryStatus(ds.Master | ds.Details)
	_ = app.Servers.AddOrUpdate(ctx, noInfo)
	time.Sleep(time.Millisecond * 1)

	gs1, _ := servers.New(net.ParseIP("1.1.1.1"), 10580, 10581)
	gs1.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    `[C=FFFFFF]Swat4[\c] [b]Server`,
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
		"password":    "0",
		"numplayers":  "14",
		"maxplayers":  "16",
		"round":       "4",
		"numrounds":   "5",
	}))
	gs1.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, gs1)
	time.Sleep(time.Millisecond * 5)

	gs2, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	gs2.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Another Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "CO-OP",
		"numplayers":  "4",
		"maxplayers":  "5",
	}))
	gs2.UpdateDiscoveryStatus(ds.Master | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, gs2)

	respJSON := make([]serverListSchema, 0)
	resp, _ := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers", nil,
		testutils.MustBindJSON(&respJSON),
	)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Len(t, respJSON, 2)

	svr1 := respJSON[0]
	assert.Equal(t, "Another Swat4 Server", svr1.Hostname)
	assert.Equal(t, "Another Swat4 Server", svr1.HostnamePlain)
	assert.Equal(t, "Another Swat4 Server", svr1.HostnameHTML)
	assert.Equal(t, "2.2.2.2:10480", svr1.Address)
	assert.Equal(t, "2.2.2.2", svr1.IP)
	assert.Equal(t, 10480, svr1.Port)
	assert.Equal(t, false, svr1.Passworded)
	assert.Equal(t, "SWAT 4", svr1.GameName)
	assert.Equal(t, "CO-OP", svr1.GameType)
	assert.Equal(t, "co-op", svr1.GameTypeSlug)
	assert.Equal(t, "1.1", svr1.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", svr1.MapName)
	assert.Equal(t, "a-bomb-nightclub", svr1.MapNameSlug)
	assert.Equal(t, 4, svr1.PlayerNum)
	assert.Equal(t, 5, svr1.PlayerMax)
	assert.Equal(t, 0, svr1.RoundNum)
	assert.Equal(t, 0, svr1.RoundMax)

	svr2 := respJSON[1]
	assert.Equal(t, `[C=FFFFFF]Swat4[\c] [b]Server`, svr2.Hostname)
	assert.Equal(t, "Swat4 Server", svr2.HostnamePlain)
	assert.Equal(t, `<span style="color:#FFFFFF;">Swat4</span> Server`, svr2.HostnameHTML)
	assert.Equal(t, "1.1.1.1:10580", svr2.Address)
	assert.Equal(t, "1.1.1.1", svr2.IP)
	assert.Equal(t, 10580, svr2.Port)
	assert.Equal(t, false, svr2.Passworded)
	assert.Equal(t, "SWAT 4", svr2.GameName)
	assert.Equal(t, "VIP Escort", svr2.GameType)
	assert.Equal(t, "vip-escort", svr2.GameTypeSlug)
	assert.Equal(t, "1.1", svr2.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", svr2.MapName)
	assert.Equal(t, "a-bomb-nightclub", svr2.MapNameSlug)
	assert.Equal(t, 14, svr2.PlayerNum)
	assert.Equal(t, 16, svr2.PlayerMax)
	assert.Equal(t, 4, svr2.RoundNum)
	assert.Equal(t, 5, svr2.RoundMax)
}

func TestAPI_ListServers_Filters(t *testing.T) {
	ctx := context.TODO()
	ts, app, cancel := testutils.PrepareTestServer(func(cfg *config.Config) {
		cfg.BrowserServerLiveness = time.Second * 10
	})
	defer cancel()

	vip, _ := servers.New(net.ParseIP("1.1.1.1"), 10580, 10581)
	vip.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "VIP Escort Swat4 Server",
		"hostport":    "10480",
		"gametype":    "VIP Escort",
		"gamevariant": "SWAT 4",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"password":    "0",
		"numplayers":  "16",
		"maxplayers":  "16",
	}))
	vip.UpdateDiscoveryStatus(ds.Master | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, vip)

	vip10, _ := servers.New(net.ParseIP("2.2.2.2"), 10480, 10481)
	vip10.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "VIP 1.0 Swat4 Server",
		"hostport":    "10480",
		"gametype":    "VIP Escort",
		"mapname":     "The Wolcott Projects",
		"gamevariant": "SWAT 4",
		"gamever":     "1.0",
		"password":    "0",
		"numplayers":  "16",
		"maxplayers":  "18",
	}))
	vip10.UpdateDiscoveryStatus(ds.Master | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, vip10)

	bs, _ := servers.New(net.ParseIP("3.3.3.3"), 10480, 10481)
	bs.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "BS Swat4 Server",
		"hostport":    "10480",
		"gametype":    "Barricaded Suspects",
		"mapname":     "Food Wall Restaurant",
		"gamevariant": "SWAT 4",
		"gamever":     "1.1",
		"password":    "0",
		"numplayers":  "0",
		"maxplayers":  "16",
	}))
	bs.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, bs)

	coop, _ := servers.New(net.ParseIP("4.4.4.4"), 10480, 10481)
	coop.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "COOP Swat4 Server",
		"hostport":    "10480",
		"gametype":    "CO-OP",
		"mapname":     "Food Wall Restaurant",
		"gamevariant": "SWAT 4",
		"gamever":     "1.1",
		"password":    "0",
		"numplayers":  "0",
		"maxplayers":  "5",
	}))
	coop.UpdateDiscoveryStatus(ds.Details | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, coop)

	sg, _ := servers.New(net.ParseIP("5.5.5.5"), 10480, 10481)
	sg.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "S&G Swat4 Server",
		"hostport":    "10480",
		"gametype":    "Smash And Grab",
		"gamevariant": "SWAT 4X",
		"mapname":     "-EXP- FunTime Amusements",
		"gamever":     "1.0",
		"password":    "0",
		"numplayers":  "0",
		"maxplayers":  "16",
	}))
	sg.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.NoDetails)
	_ = app.Servers.AddOrUpdate(ctx, sg)

	coopx, _ := servers.New(net.ParseIP("6.6.6.6"), 10480, 10481)
	coopx.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "TSS COOP Swat4 Server",
		"hostport":    "10480",
		"gametype":    "CO-OP",
		"gamevariant": "SWAT 4X",
		"mapname":     "-EXP- FunTime Amusements",
		"gamever":     "1.0",
		"password":    "0",
		"numplayers":  "1",
		"maxplayers":  "10",
	}))
	coopx.UpdateDiscoveryStatus(ds.Master | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, coopx)

	passworded, _ := servers.New(net.ParseIP("7.7.7.7"), 10480, 10481)
	passworded.UpdateInfo(details.MustNewInfoFromParams(map[string]string{
		"hostname":    "Private Swat4 Server",
		"hostport":    "10480",
		"gametype":    "VIP Escort",
		"gamevariant": "SWAT 4",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"password":    "1",
		"numplayers":  "0",
		"maxplayers":  "16",
	}))
	passworded.UpdateDiscoveryStatus(ds.Details | ds.Info)
	_ = app.Servers.AddOrUpdate(ctx, passworded)

	tests := []struct {
		name    string
		qs      url.Values
		servers []string
	}{
		{
			"no filters applied",
			url.Values{},
			[]string{
				"Private Swat4 Server", "TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide passworded",
			url.Values{"nopassworded": []string{"1"}},
			[]string{
				"TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide full",
			url.Values{"nofull": []string{"1"}},
			[]string{
				"Private Swat4 Server", "TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server",
			},
		},
		{
			"hide empty",
			url.Values{"noempty": []string{"1"}},
			[]string{
				"TSS COOP Swat4 Server", "VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide full and hide empty",
			url.Values{"nofull": []string{"1"}, "noempty": []string{"1"}},
			[]string{
				"TSS COOP Swat4 Server", "VIP 1.0 Swat4 Server",
			},
		},
		{
			"coop",
			url.Values{"gametype": []string{"CO-OP"}},
			[]string{
				"TSS COOP Swat4 Server", "COOP Swat4 Server",
			},
		},
		{
			"coop not empty",
			url.Values{"gametype": []string{"CO-OP"}, "noempty": []string{"1"}},
			[]string{
				"TSS COOP Swat4 Server",
			},
		},
		{
			"coop not empty and not full",
			url.Values{"gametype": []string{"CO-OP"}, "noempty": []string{"1"}, "nofull": []string{"1"}},
			[]string{
				"TSS COOP Swat4 Server",
			},
		},
		{
			"coop 1.0",
			url.Values{"gametype": []string{"CO-OP"}, "gamever": []string{"1.0"}, "gamevariant": []string{"SWAT 4"}},
			[]string{},
		},
		{
			"coop 1.1",
			url.Values{"gametype": []string{"CO-OP"}, "gamever": []string{"1.1"}, "gamevariant": []string{"SWAT 4"}},
			[]string{"COOP Swat4 Server"},
		},
		{
			"coop tss",
			url.Values{"gametype": []string{"CO-OP"}, "gamever": []string{"1.0"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{"TSS COOP Swat4 Server"},
		},
		{
			"tss",
			url.Values{"gamevariant": []string{"SWAT 4X"}},
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"vip tss",
			url.Values{"gametype": []string{"VIP-Escort"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{},
		},
		{
			"tss hide empty",
			url.Values{"noempty": []string{"1"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{"TSS COOP Swat4 Server"},
		},
		{
			"tss hide full",
			url.Values{"nofull": []string{"1"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"1.1",
			url.Values{"gamever": []string{"1.1"}},
			[]string{
				"Private Swat4 Server", "COOP Swat4 Server",
				"BS Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"1.0",
			url.Values{"gamever": []string{"1.0"}},
			[]string{
				"TSS COOP Swat4 Server",
				"S&G Swat4 Server",
				"VIP 1.0 Swat4 Server",
			},
		},
		{
			"1.1 vanilla",
			url.Values{"gamever": []string{"1.1"}, "gamevariant": []string{"SWAT 4"}},
			[]string{
				"Private Swat4 Server", "COOP Swat4 Server",
				"BS Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"1.0 vanilla",
			url.Values{"gamever": []string{"1.0"}, "gamevariant": []string{"SWAT 4"}},
			[]string{"VIP 1.0 Swat4 Server"},
		},
		{
			"1.0 tss",
			url.Values{"gamever": []string{"1.0"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"1.1 tss",
			url.Values{"gamever": []string{"1.1"}, "gamevariant": []string{"SWAT 4X"}},
			[]string{},
		},
		{
			"1.1 vanilla sg",
			url.Values{
				"gamever":     []string{"1.1"},
				"gamevariant": []string{"SWAT 4"},
				"gametype":    []string{"Smash And Grab"},
			},
			[]string{},
		},
		{
			"unknown gamevariant",
			url.Values{"gamevariant": []string{"Invalid"}},
			[]string{},
		},
		{
			"unknown gametype",
			url.Values{"gametype": []string{"Unknown"}},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respJSON := make([]serverListSchema, 0)
			uri := "/api/servers"
			if len(tt.qs) > 0 {
				uri = fmt.Sprintf("%s?%s", uri, tt.qs.Encode())
			}
			resp, _ := testutils.DoTestRequest(
				ts, http.MethodGet, uri, nil,
				testutils.MustBindJSON(&respJSON),
			)
			resp.Body.Close()
			assert.Equal(t, 200, resp.StatusCode)

			actualNames := make([]string, 0, len(respJSON))
			for _, svr := range respJSON {
				actualNames = append(actualNames, svr.Hostname)
			}

			assert.Len(t, respJSON, len(tt.servers))
			assert.Equal(t, tt.servers, actualNames)
		})
	}
}

func TestAPI_ListServers_Empty(t *testing.T) {
	ts, _, cancel := testutils.PrepareTestServer()
	defer cancel()

	respJSON := make([]serverListSchema, 0)
	resp, _ := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers", nil,
		testutils.MustBindJSON(&respJSON),
	)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Len(t, respJSON, 0)
}
