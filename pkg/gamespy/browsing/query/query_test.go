package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
)

func TestQuery_Parse_OK(t *testing.T) {
	tests := []string{
		"mapname='A-Bomb Nightclub'",
		"mapname='A-Bomb Nightclub' and mapname='Food Wall Restaurant' and mapname='-EXP- FunTime Amusements'",
		"gametype='CO-OP' and gamever='1.1'",
		"numplayers!=maxplayers and password=0 and gamever='1.1' and gamevariant='SWAT 4'",
		"numplayers>0 and numplayers!=maxplayers",
		"numplayers>0 and numplayers>0 and numplayers>0",
		"numplayers=maxplayers",
		"numplayers>hostport",
	}
	for _, testQ := range tests {
		_, err := query.NewFromString(testQ)
		assert.NoError(t, err)
	}
}

func TestQuery_Parse_Error(t *testing.T) {
	tests := []string{
		"",
		"mapname=A-Bomb Nightclub'",
		"mapname='A-Bomb Nightclub",
		"foo='A-Bomb Nightclub' and bar='CO-OP'",
		"gametype='CO-OP' and '1.1'",
		"mapname=A-Bomb Nightclub",
		"mapname = 'A-Bomb Nightclub'",
		"mapname = ''",
		"mapname=''",
		"mapname",
		"numplayers>foobar",
		"numplayers!=publicip",
		">10",
		"10>numplayers>20",
		"!=numplayers>",
		"10!=20",
		"natneg=0",
		"localip0='192.168.1.1'",
		"localport=10481",
		"and",
		"and ",
		" and",
		" and ",
		"and and",
		" and and",
		"and and ",
		" and and ",
		"  and  ",
		"numplayers=0  and ",
	}
	for _, testQ := range tests {
		_, err := query.NewFromString(testQ)
		assert.Error(t, err)
	}
}

func TestQuery_ParseAndMatch(t *testing.T) {
	tests := []struct {
		filters string
		want    bool
	}{
		{"mapname='A-Bomb Nightclub'", false},
		{"mapname='Food Wall Restaurant'", true},
		{"mapname='A-Bomb Nightclub' and mapname='Food Wall Restaurant' and mapname='-EXP- FunTime Amusements'", false},
		{"gametype='CO-OP' and gamever='1.1'", false},
		{"gametype='Rapid Deployment' and gamever='1.1'", true},
		{"gametype='Rapid Deployment' and gamever='1.0'", false},
		{"numplayers!=maxplayers and password=0 and gamever='1.1' and gamevariant='SWAT 4'", false},
		{"numplayers!=maxplayers and password=1 and gamever='1.1' and gamevariant='SWAT 4'", true},
		{"numplayers>0 and numplayers!=maxplayers", true},
		{"numplayers!=maxplayers and numplayers=maxplayers", false},
		{"numplayers>15", false},
		{"numplayers>14", true},
		{"numplayers<15", false},
		{"numplayers<16", true},
		{"numplayers>14 and numplayers<16", true},
		{"numplayers>14 and numplayers<14", false},
		{"numplayers>0 and numplayers>0 and numplayers>0", true},
		{"numplayers=maxplayers", false},
		{"numplayers!=maxplayers", true},
		{"numplayers>maxplayers", false},
		{"numplayers<maxplayers", true},
		{"maxplayers<numplayers", false},
		{"maxplayers>numplayers", true},
		{"numplayers!=gamever", false}, // mismatching types
		{"numplayers>gamever", false},  // mismatching types
		{"numplayers!=maxplayers and numplayers<16", true},
		{"numplayers!=maxplayers and numplayers<15", false},
		{"hostport=10480", true},
		{"hostport='10480'", false}, // unexpected type
	}

	type Schema struct {
		Hostname     string
		HostPort     int
		JoinPort     int `param:"gameport"` // fake field, just to test equality operator
		GameVariant  string
		GameVersion  string `param:"gamever"`
		GameName     string
		GameType     string
		NumPlayers   int
		MaxPlayers   int
		MapName      string
		Password     bool
		StatsEnabled bool
	}

	for _, tt := range tests {
		t.Run(tt.filters, func(t *testing.T) {
			q, err := query.NewFromString(tt.filters)
			require.NoError(t, err)

			fields := Schema{
				Hostname:     "Swat4 Server",
				HostPort:     10480,
				JoinPort:     10480,
				GameVariant:  "SWAT 4",
				GameVersion:  "1.1",
				GameName:     "swat4",
				GameType:     "Rapid Deployment",
				NumPlayers:   15,
				MaxPlayers:   16,
				MapName:      "Food Wall Restaurant",
				Password:     true,
				StatsEnabled: false,
			}

			match := q.Match(&fields)
			assert.Equal(t, tt.want, match)
		})
	}
}
