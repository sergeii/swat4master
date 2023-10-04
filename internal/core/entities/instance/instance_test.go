package instance_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/internal/core/entities/instance"
)

func TestInstance_New(t *testing.T) {
	ins, err := instance.New("foo", net.ParseIP("2.2.2.2"), 10480)
	assert.NoError(t, err)
	assert.Equal(t, "foo", ins.ID)
	assert.Equal(t, "2.2.2.2", ins.Addr.GetDottedIP())
	assert.Equal(t, 10480, ins.Addr.Port)
	assert.Equal(t, "2.2.2.2:10480", ins.Addr.String())
}
