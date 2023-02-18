package params_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

type TestSchema1 struct {
	Hostname      string
	HostPort      int
	Password      bool
	GameType      string
	Round         int
	StatsEnabled  bool
	VIPEscaped    bool
	BombsExploded bool
	Map           string `param:"mapname"`
	gameVariant   string // unexported
	tocReports    string `param:"tocreports"` // also unexported
}

type TestSchema2 struct {
	Hostname     string
	SwatScore    int
	Password     bool
	StatsEnabled bool
}

func TestParamsUnmarshal_OK(t *testing.T) {
	schema := TestSchema1{}
	data := map[string]string{
		"hostname":      "Some Server",
		"mapname":       "A-Bomb Nightclub",
		"hostport":      "10480",
		"password":      "1",
		"vipescaped":    "false",
		"bombsexploded": "true",
		"gamevariant":   "swat4",
		"tocreports":    "12/12",
	}

	err := params.Unmarshal(data, &schema)
	assert.NoError(t, err)

	assert.Equal(t, "Some Server", schema.Hostname)
	assert.Equal(t, 10480, schema.HostPort)
	assert.Equal(t, true, schema.Password)
	assert.Equal(t, "A-Bomb Nightclub", schema.Map)
	assert.Equal(t, false, schema.VIPEscaped)
	assert.Equal(t, true, schema.BombsExploded)
	assert.Equal(t, "", schema.GameType)
	assert.Equal(t, 0, schema.Round)
	assert.Equal(t, false, schema.StatsEnabled)
	assert.Equal(t, "", schema.gameVariant)
	assert.Equal(t, "", schema.tocReports)
}

func TestParamsUnmarshal_ValueErrors(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]string
		wantErr    bool
		wantErrMsg string
	}{
		{
			"general positive case",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "3",
				"password":     "1",
				"statsenabled": "true",
			},
			false,
			"",
		},
		{
			"positive case - negative integer",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "-10",
				"password":     "1",
				"statsenabled": "true",
			},
			false,
			"",
		},
		{
			"positive case - empty string",
			map[string]string{
				"hostname":     "",
				"swatscore":    "10",
				"password":     "1",
				"statsenabled": "true",
			},
			false,
			"",
		},
		{
			"bad integer value #1",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "foo",
				"password":     "1",
				"statsenabled": "true",
			},
			true,
			"invalid value 'foo' for integer field 'SwatScore'",
		},
		{
			"bad integer value #2",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "1.0",
				"password":     "1",
				"statsenabled": "true",
			},
			true,
			"invalid value '1.0' for integer field 'SwatScore'",
		},
		{
			"bad boolean value #1",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "10",
				"password":     "3",
				"statsenabled": "true",
			},
			true,
			"invalid value '3' for boolean field 'Password'",
		},
		{
			"bad boolean value #2",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "10",
				"password":     "-1",
				"statsenabled": "true",
			},
			true,
			"invalid value '-1' for boolean field 'Password'",
		},
		{
			"bad boolean value #3",
			map[string]string{
				"hostname":     "Swat4 Server",
				"swatscore":    "10",
				"password":     "yes",
				"statsenabled": "true",
			},
			true,
			"invalid value 'yes' for boolean field 'Password'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := TestSchema2{}
			err := params.Unmarshal(tt.data, &schema)
			if tt.wantErr {
				assert.ErrorContains(t, err, tt.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
