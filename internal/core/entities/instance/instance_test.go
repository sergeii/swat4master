package instance_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/instance"
)

func TestIdentifier_New_OK(t *testing.T) {
	id, err := instance.NewID([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, "test", string(id[:]))
}

func TestIdentifier_New_Errors(t *testing.T) {
	tests := []struct {
		name string
		id   []byte
	}{
		{
			name: "empty",
			id:   []byte{},
		},
		{
			name: "short",
			id:   []byte("123"),
		},
		{
			name: "long",
			id:   []byte("12345"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := instance.NewID(tt.id)
			assert.ErrorContains(t, err, "instance ID must be 4 bytes long")
		})
	}
}

func TestInstance_New_OK(t *testing.T) {
	id := instance.MustNewID([]byte("test"))
	ins, err := instance.New(id, net.ParseIP("2.2.2.2"), 10480)
	assert.NoError(t, err)
	assert.Equal(t, id, ins.ID)
	assert.Equal(t, "2.2.2.2", ins.Addr.GetDottedIP())
	assert.Equal(t, 10480, ins.Addr.Port)
	assert.Equal(t, "2.2.2.2:10480", ins.Addr.String())
}

func TestInstance_New_Errors(t *testing.T) {
	tests := []struct {
		name       string
		id         instance.Identifier
		ip         string
		port       int
		wantErrMsg string
	}{
		{
			name:       "invalid ip",
			id:         instance.MustNewID([]byte("test")),
			ip:         "256.256.256.256",
			port:       10480,
			wantErrMsg: "invalid IP address",
		},
		{
			name:       "invalid port",
			id:         instance.MustNewID([]byte("test")),
			ip:         "1.1.1.1",
			port:       0,
			wantErrMsg: "invalid port number",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := instance.New(tt.id, net.ParseIP(tt.ip), tt.port)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErrMsg)
		})
	}
}
