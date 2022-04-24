package details

import (
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/validation"
	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

type Info struct {
	Hostname       string `validate:"required"`
	HostPort       int    `validate:"required,gt=0"`
	GameVariant    string `validate:"required"`
	GameVersion    string `validate:"required" param:"gamever"`
	GameType       string `validate:"required"`
	NumPlayers     int    `validate:"gte=0"`
	MaxPlayers     int    `validate:"gte=0"`
	MapName        string `validate:"required"`
	Password       bool
	StatsEnabled   bool
	Round          int `validate:"gte=0"`
	NumRounds      int `validate:"gte=0"`
	TimeLeft       int // can be negative
	TimeSpecial    int `validate:"gte=0"`
	SwatScore      int
	SuspectsScore  int
	SwatWon        int    `validate:"gte=0"`
	SuspectsWon    int    `validate:"gte=0"`
	BombsDefused   int    `validate:"gte=0"`
	BombsTotal     int    `validate:"gte=0"`
	TocReports     string `validate:"ratio"`
	WeaponsSecured string `validate:"ratio"`

	Version string `validate:"-" param:"-"`
}

type Details struct {
	Info       Info        `validate:"structonly" param:"-"`
	Players    []Player    `validate:"dive" param:"-"`
	Objectives []Objective `validate:"dive" param:"-"`
}

func MustNewDetailsFromParams(
	info map[string]string,
	players []map[string]string,
	objectives []map[string]string,
) Details {
	details, err := NewDetailsFromParams(info, players, objectives)
	if err != nil {
		panic(err)
	}
	return details
}

func MustNewInfoFromParams(pms map[string]string) Info {
	info, err := NewInfoFromParams(pms)
	if err != nil {
		panic(err)
	}
	return info
}

func NewDetailsFromParams(
	pms map[string]string,
	players []map[string]string,
	objectives []map[string]string,
) (Details, error) {
	details := Details{}

	if len(players) > 0 {
		details.Players = make([]Player, len(players))
	}
	for i := range players {
		if err := params.Unmarshal(players[i], &details.Players[i]); err != nil {
			log.Warn().Err(err).Msgf("Unable to unmarshal player params for %v", players[i])
			return Details{}, err
		}
	}

	if len(objectives) > 0 {
		details.Objectives = make([]Objective, len(objectives))
	}
	for i := range objectives {
		if err := params.Unmarshal(objectives[i], &details.Objectives[i]); err != nil {
			log.Warn().Err(err).Msgf("Unable to unmarshal objective params for %v", objectives[i])
			return Details{}, err
		}
	}

	info, err := NewInfoFromParams(pms)
	if err != nil {
		return Details{}, err
	}
	details.Info = info

	if err := validation.Validate.Struct(&details); err != nil {
		log.Warn().Err(err).Msg("Schema validation failed")
		return Details{}, err
	}

	return details, nil
}

func NewInfoFromParams(pms map[string]string) (Info, error) {
	info := Info{}

	if err := params.Unmarshal(pms, &info); err != nil {
		log.Warn().Err(err).Msgf("Unable to unmarshal servers params for %v", info)
		return Info{}, err
	}

	if err := validation.Validate.Struct(&info); err != nil {
		log.Warn().Err(err).Msg("Schema validation failed")
		return Info{}, err
	}

	return info, nil
}
