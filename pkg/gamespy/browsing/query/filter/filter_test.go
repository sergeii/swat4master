package filter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

func TestIsGoodField(t *testing.T) {
	tests := []struct {
		field string
		want  bool
	}{
		{
			"hostname",
			true,
		},
		{
			"hostport",
			true,
		},
		{
			"statsenabled",
			true,
		},
		{
			"statechanged",
			false,
		},
		{
			"localport",
			false,
		},
		{
			"localip0",
			false,
		},
		{
			"natneg",
			false,
		},
		{
			"foobar",
			false,
		},
		{
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			assert.Equal(t, filter.IsQueryField(tt.field), tt.want)
		})
	}
}

func TestFilter_Parse(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		want    string
		wantErr error
	}{
		{
			name:   "positive case with string value",
			filter: "gametype='VIP Escort'",
			want:   "gametype=VIP Escort",
		},
		{
			name:   "positive case with numeric value #1",
			filter: "password=0",
			want:   "password=0",
		},
		{
			name:   "positive case with numeric value #2",
			filter: "numplayers>0",
			want:   "numplayers>0",
		},
		{
			name:   "positive case with field value",
			filter: "numplayers!=maxplayers",
			want:   "numplayers!={maxplayers}",
		},
		{
			name:    "empty field name",
			filter:  "='VIP Escort'",
			wantErr: filter.ErrInvalidFilterFormat,
		},
		{
			name:    "empty filter",
			filter:  "",
			wantErr: filter.ErrInvalidFilterFormat,
		},
		{
			name:    "no field and value",
			filter:  "=",
			wantErr: filter.ErrInvalidFilterFormat,
		},
		{
			name:    "unknown operator #1",
			filter:  "gametype=='VIP Escort'",
			wantErr: filter.ErrUnsupportedOperatorType,
		},
		{
			name:    "unknown operator #2",
			filter:  "numplayers>=0",
			wantErr: filter.ErrUnsupportedOperatorType,
		},
		{
			name:    "missing operator",
			filter:  "gametype'VIP Escort'",
			wantErr: filter.ErrInvalidFilterFormat,
		},
		{
			name:    "unknown field",
			filter:  "publicip='1.1.1.1'",
			wantErr: filter.ErrUnknownFieldName,
		},
		{
			name:    "badly formatted string #1",
			filter:  "gametype=VIP Escort",
			wantErr: filter.ErrInvalidValueFormat,
		},
		{
			name:    "badly formatted string #2",
			filter:  "gametype='VIP Escort",
			wantErr: filter.ErrInvalidValueFormat,
		},
		{
			name:    "badly formatted string #3",
			filter:  "gametype=VIP Escort'",
			wantErr: filter.ErrInvalidValueFormat,
		},
		{
			name:    "badly formatted string #4",
			filter:  "gametype=\"VIP Escort\"",
			wantErr: filter.ErrInvalidValueFormat,
		},
		{
			name:    "empty string value",
			filter:  "gametype=''",
			wantErr: filter.ErrInvalidValueFormat,
		},
		{
			name:    "unknown field value",
			filter:  "numplayers!=foobar",
			wantErr: filter.ErrInvalidValueFormat,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filter.ParseFilter(tt.filter)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.Equal(t, got.String(), tt.want)
			}
		})
	}
}

func TestFilter_Match(t *testing.T) {
	type filterArgs struct {
		Field string
		Op    string
		Value interface{}
	}
	tests := []struct {
		name    string
		args    filterArgs
		want    bool
		wantErr error
	}{
		{
			name: "match against equal string",
			args: filterArgs{"gametype", "=", "Rapid Deployment"},
			want: true,
		},
		{
			name: "no match against equal string",
			args: filterArgs{"gametype", "=", "VIP Escort"},
			want: false,
		},
		{
			name: "match against unequal string",
			args: filterArgs{"gametype", "!=", "VIP Escort"},
			want: true,
		},
		{
			name: "no match against unequal string",
			args: filterArgs{"gametype", "!=", "Rapid Deployment"},
			want: false,
		},
		{
			name:    "unsupported operators for string #1",
			args:    filterArgs{"gametype", ">", "Rapid Deployment"},
			wantErr: filter.ErrFieldUnsupportedOperatorType,
		},
		{
			name:    "unsupported operators for string #2",
			args:    filterArgs{"gametype", "<", "Rapid Deployment"},
			wantErr: filter.ErrFieldUnsupportedOperatorType,
		},
		{
			name: "match against equal number",
			args: filterArgs{"password", "=", 1},
			want: true,
		},
		{
			name: "no match against equal number",
			args: filterArgs{"password", "=", 0},
			want: false,
		},
		{
			name: "match against unequal number",
			args: filterArgs{"password", "!=", 0},
			want: true,
		},
		{
			name: "no match against equal number",
			args: filterArgs{"password", "!=", 1},
			want: false,
		},
		{
			name: "match against greater number",
			args: filterArgs{"numplayers", ">", 0},
			want: true,
		},
		{
			name: "no match against greater number",
			args: filterArgs{"numplayers", ">", 15},
			want: false,
		},
		{
			name: "match against lesser number",
			args: filterArgs{"numplayers", "<", 16},
			want: true,
		},
		{
			name: "no match against lesser number",
			args: filterArgs{"numplayers", "<", 15},
			want: false,
		},
		{
			name: "no match for missing field #1",
			args: filterArgs{"statsenabled", "=", 0},
			want: false,
		},
		{
			name: "no match for missing field #2",
			args: filterArgs{"statsenabled", "!=", 1},
			want: false,
		},
		{
			name: "no match for missing field #3",
			args: filterArgs{"statsenabled", ">", 0},
			want: false,
		},
		{
			name: "match against equal field value",
			args: filterArgs{"hostport", "=", filter.NewFieldValue("gameport")},
			want: true,
		},
		{
			name: "no match against equal field value",
			args: filterArgs{"numplayers", "=", filter.NewFieldValue("maxplayers")},
			want: false,
		},
		{
			name: "match against unequal field value",
			args: filterArgs{"numplayers", "!=", filter.NewFieldValue("maxplayers")},
			want: true,
		},
		{
			name: "match against unequal string field value",
			args: filterArgs{"gamename", "!=", filter.NewFieldValue("gamevariant")},
			want: true,
		},
		{
			name: "no match against equal string field value",
			args: filterArgs{"gamename", "=", filter.NewFieldValue("gamevariant")},
			want: false,
		},
		{
			name: "match against unequal field value",
			args: filterArgs{"numplayers", "!=", filter.NewFieldValue("maxplayers")},
			want: true,
		},
		{
			name: "no match against greater field value",
			args: filterArgs{"numplayers", ">", filter.NewFieldValue("maxplayers")},
			want: false,
		},
		{
			name: "match against lesser field value",
			args: filterArgs{"numplayers", "<", filter.NewFieldValue("maxplayers")},
			want: true,
		},
		{
			name: "no match against unequal field value",
			args: filterArgs{"hostport", "!=", filter.NewFieldValue("gameport")},
			want: false,
		},
		{
			name:    "cannot match against missing field value #1",
			args:    filterArgs{"numplayers", "=", filter.NewFieldValue("statsenabled")},
			wantErr: filter.ErrFieldNotFound,
		},
		{
			name:    "cannot match against missing field value #2",
			args:    filterArgs{"numplayers", "!=", filter.NewFieldValue("statsenabled")},
			wantErr: filter.ErrFieldNotFound,
		},
		{
			name:    "compared fields must to of the same type #1",
			args:    filterArgs{"maxplayers", "!=", filter.NewFieldValue("gamever")},
			wantErr: filter.ErrFieldInvalidValueType,
		},
		{
			name:    "compared fields must to of the same type #2",
			args:    filterArgs{"maxplayers", "=", filter.NewFieldValue("gamever")},
			wantErr: filter.ErrFieldInvalidValueType,
		},
	}

	type Schema struct {
		Hostname         string
		HostPort         int
		JoinPort         int `param:"gameport"` // fake field, just to test equality operator
		GameVariant      string
		GameVersion      string `param:"gamever"`
		GameName         string
		GameType         string
		NumPlayers       int
		MaxPlayers       int
		MapName          string
		Password         bool
		AllBombsDisarmed bool
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := filter.NewFilter(tt.args.Field, tt.args.Op, tt.args.Value)
			require.NoError(t, err)

			fields := Schema{
				Hostname:         "Swat4 Server",
				HostPort:         10480,
				JoinPort:         10480,
				GameVariant:      "SWAT 4",
				GameVersion:      "1.1",
				GameName:         "swat4",
				GameType:         "Rapid Deployment",
				NumPlayers:       15,
				MaxPlayers:       16,
				MapName:          "Food Wall Restaurant",
				Password:         true,
				AllBombsDisarmed: false,
			}

			match, matchErr := f.Match(&fields)
			if tt.wantErr != nil {
				assert.ErrorIs(t, matchErr, tt.wantErr)
				assert.Equal(t, false, match)
			} else {
				assert.NoError(t, matchErr)
				assert.Equal(t, tt.want, match)
			}
		})
	}
}
