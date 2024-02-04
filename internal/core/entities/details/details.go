package details

import (
	"github.com/go-playground/validator/v10"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

type Details struct {
	Info       Info        `param:"-" validate:"required"`
	Players    []Player    `param:"-" validate:"dive"`
	Objectives []Objective `param:"-" validate:"dive"`
}

var Blank Details

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
			return Details{}, err
		}
	}

	if len(objectives) > 0 {
		details.Objectives = make([]Objective, len(objectives))
	}
	for i := range objectives {
		if err := params.Unmarshal(objectives[i], &details.Objectives[i]); err != nil {
			return Details{}, err
		}
	}

	info, err := NewInfoFromParams(pms)
	if err != nil {
		return Details{}, err
	}
	details.Info = info

	return details, nil
}

func (d Details) Validate(v *validator.Validate) error {
	return v.Struct(&d)
}
