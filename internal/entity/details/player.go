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
	Name            string           `validate:"required" param:"player"`
	Score           int              `param:"score"`
	Ping            int              `param:"ping"`
	Team            PlayerTeam       `validate:"oneof=0 1 2"`
	VIP             bool             `param:"vip"`
	CoopStatus      PlayerCoopStatus `validate:"oneof=0 1 2 3 4"`
	Kills           int              `validate:"gte=0"`
	TeamKills       int              `validate:"gte=0" param:"tkills"`
	Deaths          int              `validate:"gte=0"`
	Arrests         int              `validate:"gte=0"`
	Arrested        int              `validate:"gte=0"`
	VIPEscapes      int              `validate:"gte=0" param:"vescaped"`
	VIPArrests      int              `validate:"gte=0" param:"arrestedvip"`
	VIPRescues      int              `validate:"gte=0" param:"unarrestedvip"`
	VIPKillsValid   int              `validate:"gte=0" param:"validvipkills"`
	VIPKillsInvalid int              `validate:"gte=0" param:"invalidvipkills"`
	BombsDefused    int              `validate:"gte=0" param:"bombsdiffused"`
	BombsDetonated  bool             `param:"rdcrybaby"`
	CaseEscapes     int              `validate:"gte=0" param:"escapedcase"`
	CaseKills       int              `validate:"gte=0" param:"killedcase"`
	CaseSecured     bool             `param:"sgcrybaby"`
}
