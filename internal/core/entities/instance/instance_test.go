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
	assert.Equal(t, "foo", ins.GetID())
	assert.Equal(t, "2.2.2.2", ins.GetDottedIP())
	assert.Equal(t, 10480, ins.GetAddr().Port)
	assert.Equal(t, "2.2.2.2:10480", ins.GetAddr().String())
}
