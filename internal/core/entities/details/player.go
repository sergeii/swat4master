package details

import (
	"fmt"
)

type PlayerCoopStatus int

const (
	CoopStatusUnknown PlayerCoopStatus = iota
	CoopStatusReady
	CoopStatusHealthy
	CoopStatusInjured
	CoopStatusIncapacitated
)

func (cs PlayerCoopStatus) String() string {
	switch cs { // nolint: exhaustive
	case CoopStatusUnknown:
		return "unknown"
	case CoopStatusReady:
		return "Ready"
	case CoopStatusHealthy:
		return "Healthy"
	case CoopStatusInjured:
		return "Injured"
	case CoopStatusIncapacitated:
		return "Incapacitated"
	}
	return fmt.Sprintf("%d", cs)
}

type PlayerTeam int

const (
	TeamSwat     = 0
	TeamSuspects = 1
	TeamSwatRed  = 2 // CO-OP Team Red
)

func (pt PlayerTeam) String() string {
	switch pt { // nolint: exhaustive
	case TeamSwat, TeamSwatRed:
		return "swat"
	case TeamSuspects:
		return "suspects"
	}
	return fmt.Sprintf("%d", pt)
}

type Player struct {
	Name            string           `param:"player"             validate:"required"`
	Score           int              `param:"score"`
	Ping            int              `param:"ping"`
	Team            PlayerTeam       `validate:"oneof=0 1 2"`
	VIP             bool             `param:"vip"`
	CoopStatus      PlayerCoopStatus `validate:"oneof=0 1 2 3 4"`
	Kills           int              `validate:"gte=0"`
	TeamKills       int              `param:"tkills"             validate:"gte=0"`
	Deaths          int              `validate:"gte=0"`
	Arrests         int              `validate:"gte=0"`
	Arrested        int              `validate:"gte=0"`
	VIPEscapes      int              `param:"vescaped"           validate:"gte=0"`
	VIPEscapes2     int              `param:"vipescaped"         validate:"gte=0"`
	VIPArrests      int              `param:"arrestedvip"        validate:"gte=0"`
	VIPRescues      int              `param:"unarrestedvip"      validate:"gte=0"`
	VIPKillsValid   int              `param:"validvipkills"      validate:"gte=0"`
	VIPKillsInvalid int              `param:"invalidvipkills"    validate:"gte=0"`
	BombsDefused    int              `param:"bombsdiffused"      validate:"gte=0"`
	BombsDetonated  bool             `param:"rdcrybaby"`
	CaseEscapes     int              `param:"escapedcase"        validate:"gte=0"`
	CaseKills       int              `param:"killedcase"         validate:"gte=0"`
	CaseSecured     bool             `param:"sgcrybaby"`
}
