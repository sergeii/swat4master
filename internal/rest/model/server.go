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

func NewServerFromDomain(s server.Server) Server {
	hostname := s.Info.Hostname
	return Server{
		Address:        s.Addr.String(),
		IP:             s.Addr.GetDottedIP(),
		Port:           s.Addr.Port,
		Hostname:       hostname,
		HostnamePlain:  styles.Clean(hostname),
		HostnameHTML:   styles.ToHTML(hostname),
		Passworded:     s.Info.Password,
		GameName:       s.Info.GameVariant,
		GameVer:        s.Info.GameVersion,
		GameType:       s.Info.GameType,
		GameTypeSlug:   slug.Make(s.Info.GameType),
		MapName:        s.Info.MapName,
		MapNameSlug:    slug.Make(s.Info.MapName),
		PlayerNum:      s.Info.NumPlayers,
		PlayerMax:      s.Info.MaxPlayers,
		RoundNum:       s.Info.Round,
		RoundMax:       s.Info.NumRounds,
		TimeLeft:       s.Info.TimeLeft,
		TimeSpecial:    s.Info.TimeSpecial,
		SwatScore:      s.Info.SwatScore,
		SuspectsScore:  s.Info.SuspectsScore,
		SwatWon:        s.Info.SwatWon,
		SuspectsWon:    s.Info.SuspectsWon,
		BombsDefused:   s.Info.BombsDefused,
		BombsTotal:     s.Info.BombsTotal,
		TocReports:     s.Info.TocReports,
		WeaponsSecured: s.Info.WeaponsSecured,
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

func NewServerPlayerFromDomain(player details.Player) ServerPlayer {
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

func NewServerObjectiveFromDomain(obj details.Objective) ServerObjective {
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

func NewServerDetailFromDomain(svr server.Server) ServerDetail {
	var objectives []ServerObjective
	var players []ServerPlayer

	if len(svr.Details.Players) > 0 {
		players = make([]ServerPlayer, 0, len(svr.Details.Players))
		for _, player := range svr.Details.Players {
			players = append(players, NewServerPlayerFromDomain(player))
		}
	}

	if len(svr.Details.Objectives) > 0 {
		objectives = make([]ServerObjective, 0, len(svr.Details.Objectives))
		for _, obj := range svr.Details.Objectives {
			objectives = append(objectives, NewServerObjectiveFromDomain(obj))
		}
	}

	return ServerDetail{
		Info:       NewServerFromDomain(svr),
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
