package details_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	"github.com/sergeii/swat4master/internal/validation"
)

func TestDetailsFromParams_OK(t *testing.T) {
	tests := []struct {
		name         string
		serverParams map[string]string
		playerParams []map[string]string
		objParams    []map[string]string
		want         details.Details
	}{
		{
			"no players no objectives",
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
			nil,
			nil,
			details.Details{
				Info: details.Info{
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
				Players:    nil,
				Objectives: nil,
			},
		},
		{
			"with players",
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
			[]map[string]string{
				{
					"ping": "155", "player": "{FAB}Nikki_Sixx<CPL>", "score": "0", "team": "1",
				},
				{
					"deaths": "1", "ping": "127", "player": "Nico^Elite", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "kills": "1", "ping": "263", "player": "Balls", "score": "1", "team": "1",
				},
				{
					"ping": "163", "player": "«|FAL|Üîîä^", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "ping": "111", "player": "Reynolds", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "ping": "117", "player": "4Taws", "score": "0", "team": "0",
				},
				{
					"ping": "142", "player": "Daro", "score": "0", "team": "1",
				},
			},
			nil,
			details.Details{
				Info: details.Info{
					Hostname:      "{FAB} Clan Server",
					HostPort:      10480,
					GameType:      "VIP Escort",
					MapName:       "Red Library Offices",
					GameVariant:   "SWAT 4",
					GameVersion:   "1.0",
					NumPlayers:    7,
					MaxPlayers:    16,
					Round:         5,
					NumRounds:     5,
					SwatScore:     41,
					SuspectsScore: 36,
					SwatWon:       1,
					SuspectsWon:   2,
				},
				Players: []details.Player{
					{
						Name: "{FAB}Nikki_Sixx<CPL>",
						Team: 1,
						Ping: 155,
					},
					{
						Name:   "Nico^Elite",
						Ping:   127,
						Deaths: 1,
					},
					{
						Name:   "Balls",
						Score:  1,
						Team:   1,
						Kills:  1,
						Deaths: 1,
						Ping:   263,
					},
					{
						Name: "«|FAL|Üîîä^",
						Ping: 163,
					},
					{
						Name:   "Reynolds",
						Ping:   111,
						Deaths: 1,
					},
					{
						Name:   "4Taws",
						Deaths: 1,
						Ping:   117,
					},
					{
						Name:  "Daro",
						Score: 0,
						Ping:  142,
						Team:  1,
					},
				},
				Objectives: nil,
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
			[]map[string]string{
				{
					"player": "Navis",
					"score":  "15",
					"ping":   "56",
				},
				{
					"player": "TAMAL(SPEC)",
					"score":  "0",
					"ping":   "160",
				},
				{
					"player": "Player",
					"score":  "3",
					"ping":   "256",
				},
				{
					"player": "Osanda(VIEW)",
					"score":  "-3",
					"ping":   "262",
				},
			},
			nil,
			details.Details{
				Info: details.Info{
					Hostname:    "[C=FFFF00]WWW.HOUSEOFPAiN.TK (Antics)",
					HostPort:    10480,
					GameType:    "Barricaded Suspects",
					MapName:     "The Wolcott Projects",
					GameVariant: "SWAT 4",
					GameVersion: "1.0",
					NumPlayers:  4,
					MaxPlayers:  12,
				},
				Players: []details.Player{
					{
						Name:  "Navis",
						Score: 15,
						Ping:  56,
					},
					{
						Name:  "TAMAL(SPEC)",
						Score: 0,
						Ping:  160,
					},
					{
						Name:  "Player",
						Score: 3,
						Ping:  256,
					},
					{
						Name:  "Osanda(VIEW)",
						Score: -3,
						Ping:  262,
					},
				},
				Objectives: nil,
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
			[]map[string]string{
				{
					"player": "Frost",
					"score":  "0",
					"ping":   "183",
				},
				{
					"player": "415",
					"score":  "0",
					"ping":   "116",
				},
				{
					"player": "user",
					"score":  "0",
					"ping":   "128",
				},
			},
			nil,
			details.Details{
				Info: details.Info{
					Hostname:    "[C=8BC34A]PlayBCM.net[C=FF5722] SEF[C=FFFFFF] [Custom Maps]",
					HostPort:    10480,
					GameType:    "CO-OP",
					MapName:     "Northside Vending",
					GameVariant: "SEF",
					GameVersion: "7.0",
					NumPlayers:  3,
					MaxPlayers:  16,
				},
				Players: []details.Player{
					{
						Name: "Frost",
						Ping: 183,
					},
					{
						Name: "415",
						Ping: 116,
					},
					{
						Name: "user",
						Ping: 128,
					},
				},
				Objectives: nil,
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
			[]map[string]string{
				{
					"player":     "Soup",
					"score":      "0",
					"team":       "1",
					"ping":       "65",
					"coopstatus": "2",
				},
				{
					"player":     "McDuffin",
					"score":      "0",
					"ping":       "90",
					"team":       "2",
					"coopstatus": "3",
				},
			},
			[]map[string]string{
				{
					"name":   "obj_Neutralize_All_Enemies",
					"status": "0",
				},
				{
					"name":   "obj_Rescue_All_Hostages",
					"status": "2",
				},
				{
					"name":   "obj_Rescue_Sterling",
					"status": "0",
				},
				{
					"name":   "obj_Neutralize_TerrorLeader",
					"status": "0",
				},
				{
					"name":   "obj_Secure_Briefcase",
					"status": "1",
				},
			},
			details.Details{
				Info: details.Info{
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
				Players: []details.Player{
					{
						Name:       "Soup",
						Ping:       65,
						Team:       1,
						CoopStatus: 2,
					},
					{
						Name:       "McDuffin",
						Ping:       90,
						Team:       2,
						CoopStatus: 3,
					},
				},
				Objectives: []details.Objective{
					{
						Name: "obj_Neutralize_All_Enemies",
					},
					{
						Name:   "obj_Rescue_All_Hostages",
						Status: 2,
					},
					{
						Name: "obj_Rescue_Sterling",
					},
					{
						Name: "obj_Neutralize_TerrorLeader",
					},
					{
						Name:   "obj_Secure_Briefcase",
						Status: 1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := details.NewDetailsFromParams(tt.serverParams, tt.playerParams, tt.objParams)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetailsFromParams_ValidateTypes(t *testing.T) {
	tests := []struct {
		name         string
		serverParams map[string]string
		playerParams []map[string]string
		objParams    []map[string]string
		wantErr      bool
		wantErrMsg   string
	}{
		{
			"case secured can not be greater than 1",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "Smash And Grab",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player":    "{FAB}Nikki_Sixx<CPL>",
					"score":     "-18",
					"ping":      "192",
					"sgcrybaby": "2",
					"team":      "1",
				},
			},
			nil,
			true,
			"invalid value '2' for boolean field 'CaseSecured'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := details.NewDetailsFromParams(tt.serverParams, tt.playerParams, tt.objParams)
			if tt.wantErr {
				assert.ErrorContains(t, err, tt.wantErrMsg)
				assert.Equal(t, "", got.Info.Hostname)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, "", got.Info.Hostname)
			}
		})
	}
}

func TestDetailsFromParams_ValidateValues(t *testing.T) {
	tests := []struct {
		name         string
		serverParams map[string]string
		playerParams []map[string]string
		objParams    []map[string]string
		wantErr      error
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
			[]map[string]string{
				{
					"ping": "155", "player": "{FAB}Nikki_Sixx<CPL>", "score": "0", "team": "1",
				},
				{
					"deaths": "1", "ping": "127", "player": "Nico^Elite", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "kills": "1", "ping": "263", "player": "Balls", "score": "1", "team": "1",
				},
				{
					"ping": "163", "player": "«|FAL|Üîîä^", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "ping": "111", "player": "Reynolds", "score": "0", "team": "0",
				},
				{
					"deaths": "1", "ping": "117", "player": "4Taws", "score": "0", "team": "0",
				},
				{
					"ping": "142", "player": "Daro", "score": "0", "team": "1",
				},
			},
			nil,
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
			nil,
			nil,
		},
		{
			"edge case - ping is optional",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player": "{FAB}Nikki_Sixx<CPL>", "score": "0", "team": "1",
				},
			},
			nil,
			nil,
		},
		{
			"edge case - ping can be zero",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player": "{FAB}Nikki_Sixx<CPL>", "score": "0", "team": "1", "ping": "0",
				},
			},
			nil,
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
			nil,
			nil,
		},
		{
			"edge case - score can be negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player": "{FAB}Nikki_Sixx<CPL>", "score": "-18", "team": "1", "ping": "192",
				},
			},
			nil,
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
			nil,
			nil,
			validator.ValidationErrors{},
		},
		{
			"coop status cannot be negative",
			map[string]string{
				"hostname":    "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player":     "Soup",
					"score":      "0",
					"team":       "1",
					"ping":       "65",
					"coopstatus": "-1",
				},
			},
			nil,
			validator.ValidationErrors{},
		},
		{
			"coop status cannot be more than 4",
			map[string]string{
				"hostname":    "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player":     "Soup",
					"score":      "0",
					"team":       "1",
					"ping":       "65",
					"coopstatus": "5",
				},
			},
			nil,
			validator.ValidationErrors{},
		},
		{
			"objective status cannot be negative",
			map[string]string{
				"hostname":    "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
			},
			nil,
			[]map[string]string{
				{
					"name":   "obj_Neutralize_All_Enemies",
					"status": "-1",
				},
			},
			validator.ValidationErrors{},
		},
		{
			"objective status cannot greater than 2",
			map[string]string{
				"hostname":    "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
			},
			nil,
			[]map[string]string{
				{
					"name":   "obj_Neutralize_All_Enemies",
					"status": "3",
				},
			},
			validator.ValidationErrors{},
		},
		{
			"objective name can not be empty",
			map[string]string{
				"hostname":    "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
			},
			nil,
			[]map[string]string{
				{
					"name":   "",
					"status": "2",
				},
			},
			validator.ValidationErrors{},
		},
		{
			"kills can not be negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player": "{FAB}Nikki_Sixx<CPL>",
					"score":  "-18",
					"ping":   "192",
					"kills":  "-1",
					"team":   "1",
				},
			},
			nil,
			validator.ValidationErrors{},
		},
		{
			"bombs defused can not be negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "Rapid Deployment",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player":        "{FAB}Nikki_Sixx<CPL>",
					"score":         "-18",
					"ping":          "192",
					"bombsdiffused": "-1",
					"team":          "1",
				},
			},
			nil,
			validator.ValidationErrors{},
		},
		{
			"bombs defused can not be negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
			},
			[]map[string]string{
				{
					"player": "{FAB}Nikki_Sixx<CPL>",
					"score":  "-18",
					"ping":   "192",
					"team":   "3",
				},
			},
			nil,
			validator.ValidationErrors{},
		},
		{
			"swatwon can not be negative",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "VIP Escort",
				"mapname":     "Red Library Offices",
				"gamevariant": "SWAT 4",
				"gamever":     "1.0",
				"swatwon":     "-1",
			},
			nil,
			nil,
			validator.ValidationErrors{},
		},
		{
			"tocreports must be valid ratio #1",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
				"tocreports":  "10",
			},
			nil,
			nil,
			validator.ValidationErrors{},
		},
		{
			"tocreports must be valid ratio #2",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
				"tocreports":  "10/",
			},
			nil,
			nil,
			validator.ValidationErrors{},
		},
		{
			"tocreports must be valid ratio #3",
			map[string]string{
				"hostname":    "{FAB} Clan Server",
				"hostport":    "10480",
				"gametype":    "CO-OP",
				"gamevariant": "SEF",
				"mapname":     "Northside Vending",
				"gamever":     "1.0",
				"tocreports":  "foo/bar",
			},
			nil,
			nil,
			validator.ValidationErrors{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validate, _ := validation.New()
			got, err := details.NewDetailsFromParams(tt.serverParams, tt.playerParams, tt.objParams)
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
