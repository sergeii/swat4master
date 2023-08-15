package details

import (
	"github.com/go-playground/validator/v10"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

type Info struct {
	Hostname       string `validate:"required"`
	HostPort       int    `validate:"required,gt=0"`
	GameVariant    string `validate:"required"`
	GameVersion    string `validate:"required" param:"gamever"` // nolint: tagalign
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

	Version string `param:"-" validate:"-"`
}

func NewInfoFromParams(pms map[string]string) (Info, error) {
	info := Info{}
	if err := params.Unmarshal(pms, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}

func MustNewInfoFromParams(pms map[string]string) Info {
	info, err := NewInfoFromParams(pms)
	if err != nil {
		panic(err)
	}
	return info
}

func (i Info) Validate(v *validator.Validate) error {
	return v.Struct(&i)
}
