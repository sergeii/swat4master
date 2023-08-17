package model

import (
	"github.com/gosimple/slug"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/pkg/swat/styles"
)

type NewServer struct {
	IP   string `binding:"required,ipv4"               json:"ip"`
	Port int    `binding:"required,gte=1025,lte=65535" json:"port"`
}

type Server struct {
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
	MapName        string `json:"mapname"` // Northside Vending, etc
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
	TocReports     string `json:"coop_reports"` // 24/28
	WeaponsSecured string `json:"coop_weapons"` // 17/19
}

func NewServerFromRepo(svr server.Server) Server {
	status := svr.GetInfo()
	hostname := status.Hostname
	return Server{
		Address:        svr.GetAddr().String(),
		IP:             svr.GetDottedIP(),
		Port:           svr.GetGamePort(),
		Hostname:       hostname,
		HostnamePlain:  styles.Clean(hostname),
		HostnameHTML:   styles.ToHTML(hostname),
		Passworded:     status.Password,
		GameName:       status.GameVariant,
		GameVer:        status.GameVersion,
		GameType:       status.GameType,
		GameTypeSlug:   slug.Make(status.GameType),
		MapName:        status.MapName,
		MapNameSlug:    slug.Make(status.MapName),
		PlayerNum:      status.NumPlayers,
		PlayerMax:      status.MaxPlayers,
		RoundNum:       status.Round,
		RoundMax:       status.NumRounds,
		TimeLeft:       status.TimeLeft,
		TimeSpecial:    status.TimeSpecial,
		SwatScore:      status.SwatScore,
		SuspectsScore:  status.SuspectsScore,
		SwatWon:        status.SwatWon,
		SuspectsWon:    status.SuspectsWon,
		BombsDefused:   status.BombsDefused,
		BombsTotal:     status.BombsTotal,
		TocReports:     status.TocReports,
		WeaponsSecured: status.WeaponsSecured,
	}
}

type ServerPlayer struct {
	Name            string `json:"name"`
	Ping            int    `json:"ping"`
	Score           int    `json:"score"`
	Team            string `json:"team"` // swat or suspects
	VIP             bool   `json:"vip"`
	CoopStatus      string `json:"coop_status"` // Healthy, etc
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
	BombsDetonated  uint8  `json:"rd_crybaby"` // yes or no
	CaseEscapes     int    `json:"sg_escapes"`
	CaseKills       int    `json:"sg_kills"`
	CaseSecured     uint8  `json:"sg_crybaby"` // yes or no
}

func NewServerPlayerFromRepo(player details.Player) ServerPlayer {
	coopStatus := player.CoopStatus.String()
	return ServerPlayer{
		Name:            player.Name,
		Ping:            player.Ping,
		Team:            player.Team.String(),
		Score:           player.Score,
		VIP:             player.VIP,
		CoopStatus:      coopStatus,
		CoopStatusSlug:  slug.Make(coopStatus),
		Kills:           player.Kills,
		TeamKills:       player.TeamKills,
		Deaths:          player.Deaths,
		Arrests:         player.Arrests,
		Arrested:        player.Arrested,
		VIPEscapes:      player.VIPEscapes,
		VIPArrests:      player.VIPArrests,
		VIPRescues:      player.VIPRescues,
		VIPKillsValid:   player.VIPKillsValid,
		VIPKillsInvalid: player.VIPKillsInvalid,
		BombsDefused:    player.BombsDefused,
		BombsDetonated:  boolToInt(player.BombsDetonated),
		CaseEscapes:     player.CaseEscapes,
		CaseKills:       player.CaseKills,
		CaseSecured:     boolToInt(player.CaseSecured),
	}
}

type ServerObjective struct {
	Name       string `json:"name"`   // obj_Investigate_Laundromat, etc
	Status     string `json:"status"` // In Progress, etc
	StatusSlug string `json:"status_slug"`
}

func NewServerObjectiveFromRepo(obj details.Objective) ServerObjective {
	status := obj.Status.String()
	return ServerObjective{
		Name:       obj.Name,
		Status:     status,
		StatusSlug: slug.Make(status),
	}
}

type ServerDetail struct {
	Info       Server            `json:"info"`
	Players    []ServerPlayer    `json:"players"`
	Objectives []ServerObjective `json:"objectives"`
}

func NewServerDetailFromRepo(svr server.Server) ServerDetail {
	var objectives []ServerObjective
	var players []ServerPlayer

	det := svr.GetDetails()

	if len(det.Players) > 0 {
		players = make([]ServerPlayer, 0, len(det.Players))
		for _, player := range det.Players {
			players = append(players, NewServerPlayerFromRepo(player))
		}
	}

	if len(det.Objectives) > 0 {
		objectives = make([]ServerObjective, 0, len(det.Objectives))
		for _, obj := range det.Objectives {
			objectives = append(objectives, NewServerObjectiveFromRepo(obj))
		}
	}

	return ServerDetail{
		Info:       NewServerFromRepo(svr),
		Players:    players,
		Objectives: objectives,
	}
}

func boolToInt(v bool) uint8 {
	switch {
	case v:
		return 1
	default:
		return 0
	}
}
