package aggregate_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/aggregate"
)

func TestServer_NewGameServer(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		port    int
		qPort   int
		want    string
		wantErr error
	}{
		{
			name:    "positive case",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   10481,
			want:    "1.1.1.1:10480",
			wantErr: nil,
		},
		{
			name:    "invalid ip address",
			ip:      "256.500.0.1",
			port:    10480,
			qPort:   10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerIP,
		},
		{
			name:    "unacceptable ip address",
			ip:      "0.0.0.0",
			port:    10480,
			qPort:   10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerIP,
		},
		{
			name:    "ipv4 address is required",
			ip:      "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			port:    10480,
			qPort:   10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerIP,
		},
		{
			name:    "valid game port number is required #1",
			ip:      "1.1.1.1",
			port:    65536,
			qPort:   10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerPort,
		},
		{
			name:    "valid game port number is required #2",
			ip:      "1.1.1.1",
			port:    -10480,
			qPort:   10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerPort,
		},
		{
			name:    "valid query port number is required #1",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   65536,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerPort,
		},
		{
			name:    "valid query port number is required #3",
			ip:      "1.1.1.1",
			port:    10480,
			qPort:   -10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerPort,
		},
		{
			name:    "valid query port number is required #3",
			ip:      "1.1.1.1",
			port:    -10480,
			qPort:   -10481,
			want:    "",
			wantErr: aggregate.ErrInvalidGameServerPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := aggregate.NewGameServer(net.ParseIP(tt.ip), tt.port, tt.qPort)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Equal(t, tt.want, got.GetAddr())
				require.Equal(t, tt.qPort, got.GetQueryPort())
				require.Equal(t, tt.ip, got.GetDottedIP())
				require.Equal(t, net.ParseIP(tt.ip).To4(), got.GetIP())
			}
		})
	}
}

func TestServer_NewGameServer_ValidIPAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			name: "positive case",
			ip:   "1.1.1.1",
			want: true,
		},
		{
			name: "invalid ip address",
			ip:   "256.500.0.1",
			want: false,
		},
		{
			name: "unspecified ip address",
			ip:   "0.0.0.0",
			want: false,
		},
		{
			name: "ipv4 address is required",
			ip:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			want: false,
		},
		{
			name: "loopback address is not accepted",
			ip:   "127.0.0.1",
			want: false,
		},
		{
			name: "multicast address is not accepted",
			ip:   "224.0.0.1",
			want: false,
		},
		{
			name: "link local broadcast address is not accepted",
			ip:   "169.254.0.1",
			want: false,
		},
		{
			name: "private network address is not accepted #1",
			ip:   "192.168.10.12",
			want: false,
		},
		{
			name: "private network address is not accepted #2",
			ip:   "10.0.0.1",
			want: false,
		},
		{
			name: "private network address is not accepted #3",
			ip:   "172.16.18.1",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := aggregate.NewGameServer(net.ParseIP(tt.ip), 10480, 10481)
			if tt.want {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, aggregate.ErrInvalidGameServerIP)
			}
		})
	}
}

func TestServer_GameServer_DefaultParams(t *testing.T) {
	svr, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	assert.Equal(t, "1.1.1.1", svr.GetDottedIP())
	params := svr.GetReportedParams()
	assert.Nil(t, params)
}

func TestServer_GameServer_ParamsAreUpdated(t *testing.T) {
	svr, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	params := svr.GetReportedParams()
	assert.Nil(t, params)
	svr.Update(map[string]string{
		"hostname": "Swat4 Server",
		"hostport": "10480",
	})
	updatedParams := svr.GetReportedParams()
	assert.Equal(t, "Swat4 Server", updatedParams["hostname"])
	assert.Equal(t, "10480", updatedParams["hostport"])
}
