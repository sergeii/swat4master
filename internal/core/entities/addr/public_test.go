package addr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
)

func TestPublicAddr(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{
			"public address",
			"147.128.88.19",
			true,
		},
		{
			"localhost",
			"127.0.0.1",
			false,
		},
		{
			"192.168.0.0/16 range",
			"192.168.1.1",
			false,
		},
		{
			"172.16.0.0/12 range",
			"172.16.128.18",
			false,
		},
		{
			"10.0.0.0/8 range",
			"10.39.1.19",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anyAddr := addr.MustNewFromDotted(tt.ip, 10480)

			publicAddr, err := addr.NewPublicAddr(anyAddr)

			if tt.want {
				assert.NoError(t, err)
				assert.Equal(t, anyAddr, publicAddr.ToAddr())
			} else {
				assert.ErrorIs(t, err, addr.ErrInvalidPublicIP)
			}
		})
	}
}
