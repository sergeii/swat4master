package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/settings"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type serverListSchema struct {
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

func TestAPI_ListServers_OK(t *testing.T) {
	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(
		t,
		fx.Decorate(func(settings settings.Settings) settings.Settings {
			settings.ServerLiveness = time.Second * 15
			return settings
		}),
	)
	defer cancel()

	// Outdated server
	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*16)),
		serverfactory.WithInfo(map[string]string{
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
		}),
	)

	// No Info status
	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master),
		serverfactory.WithRefreshedAt(time.Now()),
		serverfactory.WithInfo(map[string]string{
			"hostname":    "Cool Swat4 Server",
			"hostport":    "10480",
			"mapname":     "Riverside Training Facility",
			"gamever":     "1.0",
			"gamevariant": "SWAT 4",
			"gametype":    "Barricaded Suspects",
		}),
	)

	// Server never refreshed
	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithRandomAddress(),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
	)

	// Server with Info status and recent refresh time
	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10580),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithRefreshedAt(time.Now()),
		serverfactory.WithInfo(map[string]string{
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
		}),
	)

	// Server with Info status and recent refresh time
	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("2.2.2.2", 10480),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
		serverfactory.WithRefreshedAt(time.Now().Add(-time.Second*14)),
		serverfactory.WithInfo(map[string]string{
			"hostname":    "Another Swat4 Server",
			"hostport":    "10480",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"gamevariant": "SWAT 4",
			"gametype":    "CO-OP",
			"numplayers":  "4",
			"maxplayers":  "5",
		}),
	)

	respJSON := make([]serverListSchema, 0)
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers", nil,
		testutils.MustBindJSON(&respJSON),
	)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Len(t, respJSON, 2)

	svrByAddr := make(map[string]serverListSchema)
	for _, svr := range respJSON {
		svrByAddr[svr.Address] = svr
	}

	svr1 := svrByAddr["1.1.1.1:10580"]
	assert.Equal(t, `[C=FFFFFF]Swat4[\c] [b]Server`, svr1.Hostname)
	assert.Equal(t, "Swat4 Server", svr1.HostnamePlain)
	assert.Equal(t, `<span style="color:#FFFFFF;">Swat4</span> Server`, svr1.HostnameHTML)
	assert.Equal(t, "1.1.1.1:10580", svr1.Address)
	assert.Equal(t, "1.1.1.1", svr1.IP)
	assert.Equal(t, 10580, svr1.Port)
	assert.Equal(t, false, svr1.Passworded)
	assert.Equal(t, "SWAT 4", svr1.GameName)
	assert.Equal(t, "VIP Escort", svr1.GameType)
	assert.Equal(t, "vip-escort", svr1.GameTypeSlug)
	assert.Equal(t, "1.1", svr1.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", svr1.MapName)
	assert.Equal(t, "a-bomb-nightclub", svr1.MapNameSlug)
	assert.Equal(t, 14, svr1.PlayerNum)
	assert.Equal(t, 16, svr1.PlayerMax)
	assert.Equal(t, 4, svr1.RoundNum)
	assert.Equal(t, 5, svr1.RoundMax)
	assert.Equal(t, 877, svr1.TimeLeft)
	assert.Equal(t, 98, svr1.SwatScore)
	assert.Equal(t, 76, svr1.SuspectsScore)
	assert.Equal(t, 1, svr1.SwatWon)
	assert.Equal(t, 2, svr1.SuspectsWon)

	svr2 := svrByAddr["2.2.2.2:10480"]
	assert.Equal(t, "Another Swat4 Server", svr2.Hostname)
	assert.Equal(t, "Another Swat4 Server", svr2.HostnamePlain)
	assert.Equal(t, "Another Swat4 Server", svr2.HostnameHTML)
	assert.Equal(t, "2.2.2.2:10480", svr2.Address)
	assert.Equal(t, "2.2.2.2", svr2.IP)
	assert.Equal(t, 10480, svr2.Port)
	assert.Equal(t, false, svr2.Passworded)
	assert.Equal(t, "SWAT 4", svr2.GameName)
	assert.Equal(t, "CO-OP", svr2.GameType)
	assert.Equal(t, "co-op", svr2.GameTypeSlug)
	assert.Equal(t, "1.1", svr2.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", svr2.MapName)
	assert.Equal(t, "a-bomb-nightclub", svr2.MapNameSlug)
	assert.Equal(t, 4, svr2.PlayerNum)
	assert.Equal(t, 5, svr2.PlayerMax)
	assert.Equal(t, 0, svr2.RoundNum)
	assert.Equal(t, 0, svr2.RoundMax)
	assert.Equal(t, 0, svr2.TimeLeft)
	assert.Equal(t, 0, svr2.SwatScore)
	assert.Equal(t, 0, svr2.SuspectsScore)
	assert.Equal(t, 0, svr2.SwatWon)
	assert.Equal(t, 0, svr2.SuspectsWon)
}

func TestAPI_ListServers_Filters(t *testing.T) {
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
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(
				t,
				fx.Decorate(func(settings settings.Settings) settings.Settings {
					settings.ServerLiveness = time.Second * 10
					return settings
				}),
			)
			defer cancel()

			// VIP 1.1
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "VIP Escort Swat4 Server",
					"hostport":    "10480",
					"gametype":    "VIP Escort",
					"gamevariant": "SWAT 4",
					"mapname":     "A-Bomb Nightclub",
					"gamever":     "1.1",
					"password":    "0",
					"numplayers":  "16",
					"maxplayers":  "16",
				}),
			)

			// VIP 1.0
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "VIP 1.0 Swat4 Server",
					"hostport":    "10480",
					"gametype":    "VIP Escort",
					"mapname":     "The Wolcott Projects",
					"gamevariant": "SWAT 4",
					"gamever":     "1.0",
					"password":    "0",
					"numplayers":  "16",
					"maxplayers":  "18",
				}),
			)

			// BS 1.1
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "BS Swat4 Server",
					"hostport":    "10480",
					"gametype":    "Barricaded Suspects",
					"mapname":     "Food Wall Restaurant",
					"gamevariant": "SWAT 4",
					"gamever":     "1.1",
					"password":    "0",
					"numplayers":  "0",
					"maxplayers":  "16",
				}),
			)

			// COOP 1.1
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "COOP Swat4 Server",
					"hostport":    "10480",
					"gametype":    "CO-OP",
					"mapname":     "Food Wall Restaurant",
					"gamevariant": "SWAT 4",
					"gamever":     "1.1",
					"password":    "0",
					"numplayers":  "0",
					"maxplayers":  "5",
				}),
			)

			// S&G
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info|ds.NoDetails),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "S&G Swat4 Server",
					"hostport":    "10480",
					"gametype":    "Smash And Grab",
					"gamevariant": "SWAT 4X",
					"mapname":     "-EXP- FunTime Amusements",
					"gamever":     "1.0",
					"password":    "0",
					"numplayers":  "0",
					"maxplayers":  "16",
				}),
			)

			// COOP TSS
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Master|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "TSS COOP Swat4 Server",
					"hostport":    "10480",
					"gametype":    "CO-OP",
					"gamevariant": "SWAT 4X",
					"mapname":     "-EXP- FunTime Amusements",
					"gamever":     "1.0",
					"password":    "0",
					"numplayers":  "1",
					"maxplayers":  "10",
				}),
			)

			// VIP Escort 1.1 Passworded
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithRandomAddress(),
				serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
				serverfactory.WithRefreshedAt(time.Now()),
				serverfactory.WithInfo(map[string]string{
					"hostname":    "Private Swat4 Server",
					"hostport":    "10480",
					"gametype":    "VIP Escort",
					"gamevariant": "SWAT 4",
					"mapname":     "A-Bomb Nightclub",
					"gamever":     "1.1",
					"password":    "1",
					"numplayers":  "0",
					"maxplayers":  "16",
				}),
			)

			respJSON := make([]serverListSchema, 0)
			uri := "/api/servers"
			if len(tt.qs) > 0 {
				uri = fmt.Sprintf("%s?%s", uri, tt.qs.Encode())
			}
			resp := testutils.DoTestRequest(
				ts, http.MethodGet, uri, nil,
				testutils.MustBindJSON(&respJSON),
			)
			assert.Equal(t, 200, resp.StatusCode)

			actualNames := make([]string, 0, len(respJSON))
			for _, svr := range respJSON {
				actualNames = append(actualNames, svr.Hostname)
			}

			assert.Len(t, respJSON, len(tt.servers))
			assert.ElementsMatch(t, tt.servers, actualNames)
		})
	}
}

func TestAPI_ListServers_Empty(t *testing.T) {
	ts, cancel := testutils.PrepareTestServer(t)
	defer cancel()

	respJSON := make([]serverListSchema, 0)
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers", nil,
		testutils.MustBindJSON(&respJSON),
	)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Len(t, respJSON, 0)
}
