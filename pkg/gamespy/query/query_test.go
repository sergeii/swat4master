package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/gamespy/query"
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
		_, err := query.New(testQ)
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
	}
	for _, testQ := range tests {
		_, err := query.New(testQ)
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
		{"numplayers>0 and numplayers>0 and numplayers>0", true},
		{"numplayers=maxplayers", false},
		{"numplayers!=maxplayers", true},
		{"hostport=10480", true},
	}

	params := map[string]string{
		"gamename":     "swat4",
		"hostname":     "Swat4 Server",
		"numplayers":   "15",
		"maxplayers":   "16",
		"gametype":     "Rapid Deployment",
		"gamevariant":  "SWAT 4",
		"mapname":      "Food Wall Restaurant",
		"localport":    "10481",
		"hostport":     "10480",
		"password":     "1",
		"statsenabled": "0",
		"gamever":      "1.1",
	}
	for _, tt := range tests {
		t.Run(tt.filters, func(t *testing.T) {
			q, err := query.New(tt.filters)
			require.NoError(t, err)
			match := q.Match(params)
			assert.Equal(t, tt.want, match)
		})
	}
}
