package model

import (
	"github.com/gosimple/slug"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/pkg/swat/styles"
)

type AddServer struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type Server struct {
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

func NewServerFromRepo(svr servers.Server) Server {
	status := svr.GetInfo()
	hostname := status.Hostname
	return Server{
		Address:       svr.GetAddr().String(),
		IP:            svr.GetDottedIP(),
		Port:          svr.GetGamePort(),
		Hostname:      hostname,
		HostnamePlain: styles.Clean(hostname),
		HostnameHTML:  styles.ToHTML(hostname),
		Passworded:    status.Password,
		GameName:      status.GameVariant,
		GameVer:       status.GameVersion,
		GameType:      status.GameType,
		GameTypeSlug:  slug.Make(status.GameType),
		MapName:       status.MapName,
		MapNameSlug:   slug.Make(status.MapName),
		PlayerNum:     status.NumPlayers,
		PlayerMax:     status.MaxPlayers,
		RoundNum:      status.Round,
		RoundMax:      status.NumRounds,
	}
}
