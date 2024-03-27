package addr_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

func TestAddr(t *testing.T) {
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
			name: "private network address accepted",
			ip:   "192.168.10.12",
			want: true,
		},
		{
			name: "another private network address is accepted",
			ip:   "10.0.0.1",
			want: true,
		},
		{
			name: "loopback address is accepted",
			ip:   "127.0.0.1",
			want: true,
		},
		{
			name: "non-routable ip address is not accepted",
			ip:   "0.0.0.0",
			want: false,
		},
		{
			name: "invalid ip address",
			ip:   "256.500.0.1",
			want: false,
		},
		{
			name: "ipv4 address is required",
			ip:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := addr.New(net.ParseIP(tt.ip), 10480)
			if tt.want {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, addr.ErrInvalidIP)
			}
		})
	}
}
