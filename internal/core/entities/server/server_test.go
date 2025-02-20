package server_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/testutils/factories/infofactory"
)

func TestServer_New(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		port    int
		qPort   int
		want    addr.Addr
		wantErr error
	}{
		{
			name:    "positive case",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   10481,
			want:    addr.MustNewFromDotted("1.1.1.1", 10480),
			wantErr: nil,
		},
		{
			name:    "invalid ip address",
			ip:      "256.500.0.1",
			port:    10480,
			qPort:   10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidIP,
		},
		{
			name:    "unacceptable ip address",
			ip:      "0.0.0.0",
			port:    10480,
			qPort:   10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidIP,
		},
		{
			name:    "ipv4 address is required",
			ip:      "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			port:    10480,
			qPort:   10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidIP,
		},
		{
			name:    "valid game port number is required #1",
			ip:      "1.1.1.1",
			port:    65536,
			qPort:   10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidPort,
		},
		{
			name:    "valid game port number is required #2",
			ip:      "1.1.1.1",
			port:    -10480,
			qPort:   10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidPort,
		},
		{
			name:    "valid query port number is required #1",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   65536,
			want:    addr.Blank,
			wantErr: server.ErrInvalidQueryPort,
		},
		{
			name:    "valid query port number is required #3",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   -10481,
			want:    addr.Blank,
			wantErr: server.ErrInvalidQueryPort,
		},
		{
			name:    "valid query port number is required #3",
			ip:      "1.1.1.1",
			port:    -10480,
			qPort:   -10481,
			want:    addr.Blank,
			wantErr: addr.ErrInvalidPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := server.New(net.ParseIP(tt.ip), tt.port, tt.qPort)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Equal(t, tt.want, got.Addr)
				require.Equal(t, tt.qPort, got.QueryPort)
				require.Equal(t, tt.ip, got.Addr.GetDottedIP())
				require.Equal(t, net.ParseIP(tt.ip).To4(), got.Addr.GetIP())
				require.Equal(t, ds.New, got.DiscoveryStatus)
			}
		})
	}
}

func TestServer_New_ValidIPAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			name: "public address is accepted",
			ip:   "1.1.1.1",
			want: true,
		},
		{
			name: "loopback address is accepted",
			ip:   "127.0.0.1",
			want: true,
		},
		{
			name: "invalid ip address is not accepted",
			ip:   "256.500.0.1",
			want: false,
		},
		{
			name: "ipv6 address is not accepted",
			ip:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.New(net.ParseIP(tt.ip), 10480, 10481)
			if tt.want {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, addr.ErrInvalidIP)
			}
		})
	}
}

func TestServer_DefaultInfo(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, "", svr.Info.Hostname)
}

func TestServer_InfoIsUpdated(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	assert.Equal(t, "", svr.Info.Hostname)

	newInfo := infofactory.Build(infofactory.WithFields(
		infofactory.F{
			"hostname":    "Swat4 Server",
			"hostport":    "10480",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"gamevariant": "SWAT 4",
			"gametype":    "Barricaded Suspects",
		},
	))
	svr.UpdateInfo(newInfo)

	updatedInfo := svr.Info
	assert.Equal(t, "Swat4 Server", updatedInfo.Hostname)
	assert.Equal(t, 10480, updatedInfo.HostPort)
	assert.Equal(t, "A-Bomb Nightclub", updatedInfo.MapName)

	defaultDetails := svr.Details
	assert.Equal(t, "", defaultDetails.Info.Hostname)
	assert.Nil(t, defaultDetails.Players)
	assert.Nil(t, defaultDetails.Objectives)
}

func TestServer_DefaultDetails(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	assert.Equal(t, "1.1.1.1", svr.Addr.GetDottedIP())
	assert.Equal(t, "", svr.Details.Info.Hostname)
	assert.Nil(t, svr.Details.Players)
	assert.Nil(t, svr.Details.Objectives)
}

func TestServer_DetailsAreUpdated(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	assert.Equal(t, "", svr.Details.Info.Hostname)

	serverParams := map[string]string{
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
	}
	players := []map[string]string{
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
	}
	objectives := []map[string]string{
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
	}
	newDetails := details.MustNewDetailsFromParams(serverParams, players, objectives)
	svr.UpdateDetails(newDetails)

	updatedDetails := svr.Details
	assert.Equal(t, "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk", updatedDetails.Info.Hostname)
	assert.Equal(t, 10480, updatedDetails.Info.HostPort)
	assert.Len(t, updatedDetails.Players, 2)
	assert.Len(t, updatedDetails.Objectives, 5)

	// info is also updated
	assert.Equal(t, "[c=0099ff]SEF 7.0 EU [c=ffffff]www.swat4.tk", svr.Info.Hostname)
}

func TestServer_DiscoveryStatusIsUpdated(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

	// default status is new
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	assert.True(t, svr.HasDiscoveryStatus(ds.New))
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))

	// updating to any other status, removes the new status
	svr.UpdateDiscoveryStatus(ds.Master)
	assert.False(t, svr.HasDiscoveryStatus(ds.New))
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))

	// updates are cumulative
	svr.UpdateDiscoveryStatus(ds.Details)
	assert.False(t, svr.HasDiscoveryStatus(ds.New))
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
	assert.False(t, svr.HasDiscoveryStatus(ds.NoDetails))

	svr.UpdateDiscoveryStatus(ds.Details | ds.NoDetails)
	assert.False(t, svr.HasDiscoveryStatus(ds.New))
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
	assert.True(t, svr.HasDiscoveryStatus(ds.NoDetails))
}

func TestServer_DiscoveryStatusIsCleared(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)
	// default status is new
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	svr.ClearDiscoveryStatus(ds.New)
	assert.Equal(t, ds.DiscoveryStatus(0), svr.DiscoveryStatus)
	assert.False(t, svr.HasDiscoveryStatus(ds.New))
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))

	svr.UpdateDiscoveryStatus(ds.Master | ds.Details | ds.Info)
	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))

	svr.ClearDiscoveryStatus(ds.Master)
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
	assert.True(t, svr.HasDiscoveryStatus(ds.Info))

	// subsequent calls are idempotent
	svr.ClearDiscoveryStatus(ds.Master)
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
	assert.True(t, svr.HasDiscoveryStatus(ds.Info))

	svr.ClearDiscoveryStatus(ds.Details | ds.Info | ds.NoDetails)
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))
	assert.False(t, svr.HasDiscoveryStatus(ds.Details))
	assert.False(t, svr.HasDiscoveryStatus(ds.Info))
	assert.Equal(t, ds.NoStatus, svr.DiscoveryStatus)
}

func TestServer_HasDiscoveryStatusStatus(t *testing.T) {
	svr := server.MustNew(net.ParseIP("1.1.1.1"), 10480, 10481)

	// default status is "new"
	assert.Equal(t, ds.New, svr.DiscoveryStatus)

	assert.True(t, svr.HasDiscoveryStatus(ds.New))
	assert.False(t, svr.HasDiscoveryStatus(ds.Master))

	svr.UpdateDiscoveryStatus(ds.Master | ds.Info | ds.Details)
	assert.False(t, svr.HasDiscoveryStatus(ds.New))
	assert.Equal(t, ds.Master|ds.Info|ds.Details, svr.DiscoveryStatus)

	assert.True(t, svr.HasDiscoveryStatus(ds.Master))
	assert.True(t, svr.HasDiscoveryStatus(ds.Info))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details))
	assert.True(t, svr.HasDiscoveryStatus(ds.Details|ds.Master|ds.Info))
	assert.False(t, svr.HasDiscoveryStatus(ds.Details|ds.Master|ds.NoDetails))

	assert.True(t, svr.HasNoDiscoveryStatus(ds.NoDetails))
	assert.False(t, svr.HasNoDiscoveryStatus(ds.Details))
	assert.True(t, svr.HasNoDiscoveryStatus(ds.NoDetails|ds.NoPort))
	assert.False(t, svr.HasNoDiscoveryStatus(ds.NoDetails|ds.NoPort|ds.Details))
	assert.False(t, svr.HasNoDiscoveryStatus(ds.Details|ds.Master|ds.Info))
}
