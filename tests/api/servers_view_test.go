package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/serverfactory"
)

type serverDetailInfoSchema struct {
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

type serverDetailPlayerSchema struct {
	Name            string `json:"name"`
	Ping            int    `json:"ping"`
	Score           int    `json:"score"`
	Team            string `json:"team"`
	VIP             bool   `json:"vip"`
	CoopStatus      string `json:"coop_status"`
	CoopStatusSlug  string `json:"coop_status_slug"`
	Kills           int    `json:"kills"`
	TeamKills       int    `json:"teamkills"`
	Deaths          int    `json:"deaths"`
	Arrests         int    `json:"arrests"`
	Arrested        int    `json:"arrested"`
	VIPEscapes      int    `json:"vip_escapes"`
	VIPArrests      int    `json:"vip_captures"`
	VIPRescues      int    `json:"vip_rescues"`
	VIPKillsValid   int    `json:"vip_kills_valid"`
	VIPKillsInvalid int    `json:"vip_kills_invalid"`
	BombsDefused    int    `json:"rd_bombs_defused"`
	BombsDetonated  uint8  `json:"rd_crybaby"`
	CaseEscapes     int    `json:"sg_escapes"`
	CaseKills       int    `json:"sg_kills"`
	CaseSecured     uint8  `json:"sg_crybaby"`
}

type serverDetailObjectiveSchema struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	StatusSlug string `json:"status_slug"`
}

type serverDetailSchema struct {
	Info       serverDetailInfoSchema        `json:"info"`
	Players    []serverDetailPlayerSchema    `json:"players"`
	Objectives []serverDetailObjectiveSchema `json:"objectives"`
}

func TestAPI_ViewServer_OK(t *testing.T) {
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

	players := []map[string]string{
		{
			"ping": "155", "player": "{FAB}Nikki_Sixx<CPL>", "score": "0", "team": "1",
		},
		{
			"deaths": "1", "ping": "127", "player": "Nico^Elite", "score": "0", "team": "0",
		},
		{
			"deaths": "1", "kills": "1", "ping": "263", "player": "Balls", "score": "1", "team": "1",
		},
		{
			"ping": "163", "player": "«|FAL|Üîîä^", "score": "0", "team": "0",
		},
		{
			"deaths": "1", "ping": "111", "player": "Reynolds", "score": "0", "team": "0",
		},
		{
			"deaths": "1", "ping": "117", "player": "4Taws", "score": "0", "team": "0",
		},
		{
			"ping": "142", "player": "Daro", "score": "0", "team": "1",
		},
	}

	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10580),
		serverfactory.WithQueryPort(10581),
		serverfactory.WithDiscoveryStatus(ds.Master|ds.Details|ds.Info),
		serverfactory.WithInfo(fields),
		serverfactory.WithPlayers(players),
	)

	obj := serverDetailSchema{}
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers/1.1.1.1:10580", nil,
		testutils.MustBindJSON(&obj),
	)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, `[C=FFFFFF]Swat4[\c] [b]Server`, obj.Info.Hostname)
	assert.Equal(t, "Swat4 Server", obj.Info.HostnamePlain)
	assert.Equal(t, `<span style="color:#FFFFFF;">Swat4</span> Server`, obj.Info.HostnameHTML)
	assert.Equal(t, "1.1.1.1:10580", obj.Info.Address)
	assert.Equal(t, "1.1.1.1", obj.Info.IP)
	assert.Equal(t, 10580, obj.Info.Port)
	assert.Equal(t, false, obj.Info.Passworded)
	assert.Equal(t, "SWAT 4", obj.Info.GameName)
	assert.Equal(t, "VIP Escort", obj.Info.GameType)
	assert.Equal(t, "vip-escort", obj.Info.GameTypeSlug)
	assert.Equal(t, "1.1", obj.Info.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", obj.Info.MapName)
	assert.Equal(t, "a-bomb-nightclub", obj.Info.MapNameSlug)
	assert.Equal(t, 14, obj.Info.PlayerNum)
	assert.Equal(t, 16, obj.Info.PlayerMax)
	assert.Equal(t, 4, obj.Info.RoundNum)
	assert.Equal(t, 5, obj.Info.RoundMax)
	assert.Equal(t, 877, obj.Info.TimeLeft)
	assert.Equal(t, 98, obj.Info.SwatScore)
	assert.Equal(t, 76, obj.Info.SuspectsScore)
	assert.Equal(t, 1, obj.Info.SwatWon)
	assert.Equal(t, 2, obj.Info.SuspectsWon)

	assert.NotNil(t, obj.Players, 7)
	assert.Len(t, obj.Objectives, 0)

	player1 := obj.Players[0]
	assert.Equal(t, "{FAB}Nikki_Sixx<CPL>", player1.Name)
	assert.Equal(t, 155, player1.Ping)
	assert.Equal(t, "suspects", player1.Team)
	assert.Equal(t, 0, player1.Score)
	assert.Equal(t, 0, player1.Kills)

	player6 := obj.Players[5]
	assert.Equal(t, "4Taws", player6.Name)
	assert.Equal(t, 117, player6.Ping)
	assert.Equal(t, "swat", player6.Team)
	assert.Equal(t, 0, player6.Score)
	assert.Equal(t, 0, player6.Kills)
	assert.Equal(t, 1, player6.Deaths)
}

func TestAPI_ViewServer_Coop_OK(t *testing.T) {
	fields := map[string]string{
		"hostname":       "-==MYT Co-op Svr==-",
		"hostport":       "10880",
		"gamever":        "1.1",
		"gametype":       "CO-OP",
		"gamevariant":    "SWAT 4",
		"mapname":        "Northside Vending",
		"password":       "false",
		"numplayers":     "2",
		"maxplayers":     "5",
		"round":          "1",
		"numrounds":      "1",
		"timeleft":       "0",
		"timespecial":    "0",
		"tocreports":     "21/25",
		"weaponssecured": "1/7",
	}

	objectives := []map[string]string{
		{
			"name":   "obj_Neutralize_All_Enemies",
			"status": "0",
		},
		{
			"name":   "obj_Rescue_All_Hostages",
			"status": "0",
		},
		{
			"name":   "obj_Rescue_Baccus",
			"status": "1",
		},
		{
			"name":   "obj_Neutralize_Grover",
			"status": "0",
		},
		{
			"name":   "obj_Neutralize_Kruse",
			"status": "0",
		},
		{
			"name":   "obj_Investigate_Laundromat",
			"status": "0",
		},
		{
			"name":   "obj_Rescue_Walsh",
			"status": "0",
		},
	}

	players := []map[string]string{
		{
			"player": "Player1", "score": "0", "ping": "103", "team": "0", "coopstatus": "2",
		},
		{
			"player": "Player2", "score": "0", "ping": "44", "team": "0", "coopstatus": "3",
		},
	}

	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10880),
		serverfactory.WithQueryPort(10881),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithInfo(fields),
		serverfactory.WithPlayers(players),
		serverfactory.WithObjectives(objectives),
	)

	obj := serverDetailSchema{}
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers/1.1.1.1:10880", nil,
		testutils.MustBindJSON(&obj),
	)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, "-==MYT Co-op Svr==-", obj.Info.Hostname)
	assert.Equal(t, "-==MYT Co-op Svr==-", obj.Info.HostnamePlain)
	assert.Equal(t, "-==MYT Co-op Svr==-", obj.Info.HostnameHTML)
	assert.Equal(t, "1.1.1.1:10880", obj.Info.Address)
	assert.Equal(t, "1.1.1.1", obj.Info.IP)
	assert.Equal(t, 10880, obj.Info.Port)
	assert.Equal(t, false, obj.Info.Passworded)
	assert.Equal(t, "SWAT 4", obj.Info.GameName)
	assert.Equal(t, "CO-OP", obj.Info.GameType)
	assert.Equal(t, "co-op", obj.Info.GameTypeSlug)
	assert.Equal(t, "1.1", obj.Info.GameVer)
	assert.Equal(t, "Northside Vending", obj.Info.MapName)
	assert.Equal(t, "northside-vending", obj.Info.MapNameSlug)
	assert.Equal(t, 2, obj.Info.PlayerNum)
	assert.Equal(t, 5, obj.Info.PlayerMax)
	assert.Equal(t, 1, obj.Info.RoundNum)
	assert.Equal(t, 1, obj.Info.RoundMax)
	assert.Equal(t, 0, obj.Info.TimeLeft)
	assert.Equal(t, 0, obj.Info.SwatScore)
	assert.Equal(t, 0, obj.Info.SuspectsScore)
	assert.Equal(t, 0, obj.Info.SwatWon)
	assert.Equal(t, 0, obj.Info.SuspectsWon)
	assert.Equal(t, "21/25", obj.Info.TocReports)
	assert.Equal(t, "1/7", obj.Info.WeaponsSecured)

	assert.Len(t, obj.Players, 2)
	assert.Len(t, obj.Objectives, 7)

	player1 := obj.Players[0]
	assert.Equal(t, "Player1", player1.Name)
	assert.Equal(t, 103, player1.Ping)
	assert.Equal(t, "swat", player1.Team)
	assert.Equal(t, "Healthy", player1.CoopStatus)
	assert.Equal(t, "healthy", player1.CoopStatusSlug)
	assert.Equal(t, 0, player1.Score)

	player2 := obj.Players[1]
	assert.Equal(t, "Player2", player2.Name)
	assert.Equal(t, 44, player2.Ping)
	assert.Equal(t, "swat", player2.Team)
	assert.Equal(t, "Injured", player2.CoopStatus)
	assert.Equal(t, "injured", player2.CoopStatusSlug)
	assert.Equal(t, 0, player2.Score)
}

func TestAPI_ViewServer_MinimalInfo_OK(t *testing.T) {
	fields := map[string]string{
		"hostname":    "Swat4 Server",
		"hostport":    "10480",
		"mapname":     "A-Bomb Nightclub",
		"gamever":     "1.1",
		"gamevariant": "SWAT 4",
		"gametype":    "VIP Escort",
	}

	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithInfo(fields),
	)

	obj := serverDetailSchema{}
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers/1.1.1.1:10480", nil,
		testutils.MustBindJSON(&obj),
	)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, "Swat4 Server", obj.Info.Hostname)
	assert.Equal(t, "1.1.1.1:10480", obj.Info.Address)
	assert.Equal(t, "1.1.1.1", obj.Info.IP)
	assert.Equal(t, 10480, obj.Info.Port)
	assert.Equal(t, false, obj.Info.Passworded)
	assert.Equal(t, "SWAT 4", obj.Info.GameName)
	assert.Equal(t, "VIP Escort", obj.Info.GameType)
	assert.Equal(t, "vip-escort", obj.Info.GameTypeSlug)
	assert.Equal(t, "1.1", obj.Info.GameVer)
	assert.Equal(t, "A-Bomb Nightclub", obj.Info.MapName)
	assert.Equal(t, "a-bomb-nightclub", obj.Info.MapNameSlug)
	assert.Equal(t, 0, obj.Info.PlayerNum)
	assert.Equal(t, 0, obj.Info.PlayerMax)

	assert.Len(t, obj.Players, 0)
	assert.Len(t, obj.Objectives, 0)
}

func TestAPI_ViewServer_NoInfo_OK(t *testing.T) {
	ctx := context.TODO()
	ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
	defer cancel()

	serverfactory.Create(
		ctx,
		repos.Servers,
		serverfactory.WithAddress("1.1.1.1", 10480),
		serverfactory.WithQueryPort(10481),
		serverfactory.WithDiscoveryStatus(ds.Details|ds.Info),
		serverfactory.WithNoInfo(),
	)

	obj := serverDetailSchema{}
	resp := testutils.DoTestRequest(
		ts, http.MethodGet, "/api/servers/1.1.1.1:10480", nil,
		testutils.MustBindJSON(&obj),
	)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, "", obj.Info.Hostname)
	assert.Equal(t, "1.1.1.1:10480", obj.Info.Address)
	assert.Equal(t, "1.1.1.1", obj.Info.IP)
	assert.Equal(t, 10480, obj.Info.Port)
	assert.Equal(t, false, obj.Info.Passworded)
	assert.Equal(t, "", obj.Info.GameName)
	assert.Equal(t, "", obj.Info.GameType)
	assert.Equal(t, "", obj.Info.GameTypeSlug)
	assert.Len(t, obj.Players, 0)
	assert.Len(t, obj.Objectives, 0)
}

func TestAPI_ViewServer_NotFound(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			"positive case",
			"1.1.1.1:10480",
			true,
		},
		{
			"server not found - different port",
			"1.1.1.1:10580",
			false,
		},
		{
			"server not found - different address",
			"2.2.2.2:10580",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
			defer cancel()

			serverfactory.Create(ctx, repos.Servers, serverfactory.WithDiscoveryStatus(ds.Details))

			testPath := "/api/servers/" + tt.address

			if tt.want {
				obj := serverDetailSchema{}
				resp := testutils.DoTestRequest(
					ts, http.MethodGet, testPath, nil,
					testutils.MustBindJSON(&obj),
				)
				assert.Equal(t, 200, resp.StatusCode)
				assert.Equal(t, "Swat4 Server", obj.Info.Hostname)
			} else {
				resp := testutils.DoTestRequest(
					ts, http.MethodGet, testPath, nil,
					testutils.MustHaveNoBody(),
				)
				assert.Equal(t, 404, resp.StatusCode)
			}
		})
	}
}

func TestAPI_ViewServer_ValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			"positive case",
			"1.1.1.1:10480",
			true,
		},
		{
			"empty address parts",
			":",
			false,
		},
		{
			"no port",
			"1.1.1.1",
			false,
		},
		{
			"empty port",
			"1.1.1.1:",
			false,
		},
		{
			"negative port",
			"1.1.1.1:-1",
			false,
		},
		{
			"zero port",
			"1.1.1.1:0",
			false,
		},
		{
			"string port value",
			"1.1.1.1:foo",
			false,
		},
		{
			"float port value",
			"1.1.1.1:10480.1",
			false,
		},
		{
			"port out of range",
			"1.1.1.1:65536",
			false,
		},
		{
			"no ip address",
			"10480",
			false,
		},
		{
			"empty ip address",
			":10480",
			false,
		},
		{
			"invalid ip address",
			"300.400.500.700:10480",
			false,
		},
		{
			"local ip address",
			"127.0.0.1:10480",
			false,
		},
		{
			"private ip address",
			"192.168.1.1:10480",
			false,
		},
		{
			"v6 ip address",
			"2001:db8:3c4d:15::1a2f:1a2b:10480",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
			defer cancel()

			serverfactory.Create(ctx, repos.Servers, serverfactory.WithDiscoveryStatus(ds.Details))

			obj := serverDetailSchema{}
			resp := testutils.DoTestRequest(
				ts, http.MethodGet, "/api/servers/"+tt.address, nil,
				testutils.MustBindJSON(&obj),
			)

			if tt.want {
				assert.Equal(t, 200, resp.StatusCode)
				assert.Equal(t, "Swat4 Server", obj.Info.Hostname)
			} else {
				assert.Equal(t, 400, resp.StatusCode)
			}
		})
	}
}

func TestAPI_ViewServer_ValidateStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ds.DiscoveryStatus
		want   bool
	}{
		{
			"positive case",
			ds.Details,
			true,
		},
		{
			"no details",
			ds.Info,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ts, repos, cancel := testutils.PrepareTestServerWithRepos(t)
			defer cancel()

			fields := map[string]string{
				"hostname":    "Swat4 Server",
				"hostport":    "10480",
				"mapname":     "A-Bomb Nightclub",
				"gamever":     "1.1",
				"gamevariant": "SWAT 4",
				"gametype":    "VIP Escort",
			}
			serverfactory.Create(
				ctx,
				repos.Servers,
				serverfactory.WithAddress("1.1.1.1", 10480),
				serverfactory.WithQueryPort(10481),
				serverfactory.WithDiscoveryStatus(tt.status),
				serverfactory.WithInfo(fields),
			)

			if tt.want {
				obj := serverDetailSchema{}
				resp := testutils.DoTestRequest(
					ts, http.MethodGet, "/api/servers/1.1.1.1:10480", nil,
					testutils.MustBindJSON(&obj),
				)
				assert.Equal(t, 200, resp.StatusCode)
				assert.Equal(t, "Swat4 Server", obj.Info.Hostname)
			} else {
				resp := testutils.DoTestRequest(
					ts, http.MethodGet, "/api/servers/1.1.1.1:10480", nil,
					testutils.MustHaveNoBody(),
				)
				assert.Equal(t, 204, resp.StatusCode)
			}
		})
	}
}
