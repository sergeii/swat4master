package details_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/internal/validation"
)

func TestInfoFromParams_OK(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		want   details.Info
	}{
		{
			"general case",
			map[string]string{
				"hostname":      "-==MYT Team Svr==-",
				"hostport":      "10480",
				"password":      "false",
				"gametype":      "VIP Escort",
				"mapname":       "Fairfax Residence",
				"gamevariant":   "SWAT 4",
				"gamever":       "1.1",
				"numplayers":    "13",
				"maxplayers":    "16",
				"round":         "5",
				"numrounds":     "5",
				"timeleft":      "286",
				"timespecial":   "0",
				"swatscore":     "41",
				"suspectsscore": "36",
				"swatwon":       "1",
				"suspectswon":   "2",
				"queryid":       "3",
			},
			details.Info{
				Hostname:      "-==MYT Team Svr==-",
				HostPort:      10480,
				GameType:      "VIP Escort",
				MapName:       "Fairfax Residence",
				GameVariant:   "SWAT 4",
				GameVersion:   "1.1",
				NumPlayers:    13,
				MaxPlayers:    16,
				Round:         5,
				NumRounds:     5,
				TimeLeft:      286,
				TimeSpecial:   0,
				SwatScore:     41,
				SuspectsScore: 36,
				SwatWon:       1,
				SuspectsWon:   2,
			},
		},
		{
			"no extended params",
			map[string]string{
				"hostname":    "[C=FFFF00]WWW.HOUSEOFPAiN.TK (Antics)",
				"hostport":    "10480",
				"password":    "0",
				"gametype":    "Barricaded Suspects",
				"mapname":     "The Wolcott Projects",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
				"numplayers":  "4",
				"maxplayers":  "12",
				"queryid":     "1.1",
			},
			details.Info{
				Hostname:    "[C=FFFF00]WWW.HOUSEOFPAiN.TK (Antics)",
				HostPort:    10480,
				GameType:    "Barricaded Suspects",
				MapName:     "The Wolcott Projects",
				GameVariant: "SWAT 4",
				GameVersion: "1.0",
				NumPlayers:  4,
				MaxPlayers:  12,
			},
		},
		{
			"coop basic",
			map[string]string{
				"hostname":     "[C=8BC34A]PlayBCM.net[C=FF5722] SEF[C=FFFFFF] [Custom Maps]",
				"hostport":     "10480",
				"password":     "0",
				"gamever":      "7.0",
				"numplayers":   "3",
				"maxplayers":   "16",
				"gametype":     "CO-OP",
				"gamevariant":  "SEF",
				"mapname":      "Northside Vending",
				"statsenabled": "0",
				"queryid":      "1.1",
				"final":        "",
			},
			details.Info{
				Hostname:    "[C=8BC34A]PlayBCM.net[C=FF5722] SEF[C=FFFFFF] [Custom Maps]",
				HostPort:    10480,
				GameType:    "CO-OP",
				MapName:     "Northside Vending",
				GameVariant: "SEF",
				GameVersion: "7.0",
				NumPlayers:  3,
				MaxPlayers:  16,
			},
		},
		{
			"coop extended",
			map[string]string{
				"hostname":       "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":       "10480",
				"password":       "false",
				"gamever":        "7.0",
				"numplayers":     "2",
				"maxplayers":     "10",
				"gametype":       "CO-OP",
				"gamevariant":    "SEF",
				"mapname":        "Mt. Threshold Research Center",
				"round":          "1",
				"numrounds":      "1",
				"timeleft":       "0",
				"timespecial":    "0",
				"tocreports":     "21/25",
				"weaponssecured": "5/8",
				"queryid":        "2",
				"final":          "",
			},
			details.Info{
				Hostname:       "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				HostPort:       10480,
				GameType:       "CO-OP",
				MapName:        "Mt. Threshold Research Center",
				GameVariant:    "SEF",
				GameVersion:    "7.0",
				NumPlayers:     2,
				MaxPlayers:     10,
				Round:          1,
				NumRounds:      1,
				TocReports:     "21/25",
				WeaponsSecured: "5/8",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := details.NewInfoFromParams(tt.params)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInfoFromParams_Validation(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]string
		wantErr error
	}{
		{
			"general case",
			map[string]string{
				"hostname":      "{FAB} Clan Server",
				"hostport":      "10480",
				"password":      "0",
				"gametype":      "VIP Escort",
				"mapname":       "Red Library Offices",
				"gamevariant":   "SWAT 4",
				"gamever":       "1.0",
				"statsenabled":  "0",
				"numplayers":    "7",
				"maxplayers":    "16",
				"round":         "5",
				"numrounds":     "5",
				"swatscore":     "41",
				"suspectsscore": "36",
				"swatwon":       "1",
				"suspectswon":   "2",
			},
			nil,
		},
		{
			"edge case - zero of zero ratio",
			map[string]string{
				"hostname":       "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":       "10480",
				"gamever":        "7.0",
				"gametype":       "CO-OP",
				"gamevariant":    "SEF",
				"mapname":        "Mt. Threshold Research Center",
				"tocreports":     "21/25",
				"weaponssecured": "0/0",
			},
			nil,
		},
		{
			"edge case - swat score can be negative",
			map[string]string{
				"hostname":      "-==MYT Team Svr==-",
				"hostport":      "10480",
				"gametype":      "VIP Escort",
				"mapname":       "Fairfax Residence",
				"gamevariant":   "SWAT 4",
				"gamever":       "1.1",
				"swatscore":     "-19",
				"suspectsscore": "-2",
			},
			nil,
		},
		{
			"hostport can not negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "-10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			validator.ValidationErrors{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validate, _ := validation.New()
			got, err := details.NewInfoFromParams(tt.params)
			require.NoError(t, err)
			validateErr := got.Validate(validate)
			if tt.wantErr != nil {
				assert.Error(t, validateErr)
				assert.IsType(t, validateErr, tt.wantErr)
			} else {
				assert.NoError(t, validateErr)
			}
		})
	}
}
