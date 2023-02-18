package params_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/pkg/gamespy/serverquery/params"
)

func TestParamsMarshal_OK(t *testing.T) {
	type Schema struct {
		Hostname      string
		HostPort      int
		Password      bool
		GameType      string
		Round         int
		StatsEnabled  bool
		VIPEscaped    bool
		BombsExploded bool
		Map           string  `param:"mapname"`
		gameVariant   string  // unexported
		tocReports    string  `param:"tocreports"` // also unexported
		other         float64 // nolint: unused // unknown type, also unexported
	}
	schema := Schema{
		Hostname:      "Some Server",
		HostPort:      10480,
		Password:      true,
		Map:           "A-Bomb Nightclub",
		VIPEscaped:    false,
		BombsExploded: true,
		Round:         0,
		StatsEnabled:  false,
		gameVariant:   "swat4",
		tocReports:    "1/1",
	}
	decoded, err := params.Marshal(&schema)
	assert.NoError(t, err)

	assert.Equal(t, map[string]string{
		"hostname":      "Some Server",
		"hostport":      "10480",
		"password":      "1",
		"gametype":      "",
		"mapname":       "A-Bomb Nightclub",
		"bombsexploded": "1",
		"vipescaped":    "0",
		"round":         "0",
		"statsenabled":  "0",
	}, decoded)
}

func TestParamsMarshal_UnknownFieldType(t *testing.T) {
	type Schema struct {
		Hostname string
		HostPort float64
	}
	schema := Schema{
		Hostname: "Some Server",
		HostPort: 10480,
	}
	_, err := params.Marshal(&schema)
	assert.ErrorContains(t, err, "unable to marshal value '10480' for field 'HostPort' (unknown field type 'float64')")
}

func TestParamsMarshal_RequiresPointer(t *testing.T) {
	type Schema struct {
		Hostname string
	}
	_, err := params.Marshal(Schema{})
	assert.ErrorIs(t, err, params.ErrValueMustBePointer)
}
