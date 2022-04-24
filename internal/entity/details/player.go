package details

type Player struct {
	Name            string `validate:"required" param:"player"`
	Score           int    `param:"score"`
	Ping            int
	Team            int  `validate:"oneof=0 1 2"`
	VIP             bool `param:"vip"`
	CoopStatus      int  `validate:"oneof=0 1 2 3 4"`
	Kills           int  `validate:"gte=0"`
	TeamKills       int  `validate:"gte=0" param:"tkills"`
	Deaths          int  `validate:"gte=0"`
	Arrests         int  `validate:"gte=0"`
	Arrested        int  `validate:"gte=0"`
	VIPEscapes      int  `validate:"gte=0" param:"vescaped"`
	VIPArrests      int  `validate:"gte=0" param:"arrestedvip"`
	VIPRescues      int  `validate:"gte=0" param:"unarrestedvip"`
	VIPKillsValid   int  `validate:"gte=0" param:"validvipkills"`
	VIPKillsInvalid int  `validate:"gte=0" param:"invalidvipkills"`
	BombsDefused    int  `validate:"gte=0" param:"bombsdiffused"`
	BombsDetonated  bool `param:"rdcrybaby"`
	CaseEscapes     int  `validate:"gte=0" param:"escapedcase"`
	CaseKills       int  `validate:"gte=0" param:"killedcase"`
	CaseSecured     bool `param:"sgcrybaby"`
}
