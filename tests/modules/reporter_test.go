package modules_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/reporter"
)

func TestReporter_Run(t *testing.T) {
	todo := context.TODO()
	app := fx.New(
		application.Module,
		fx.Provide(func() config.Config {
			return config.Config{
				ReporterListenAddr: "127.0.0.1:33811",
				ReporterBufferSize: 1024,
			}
		}),
		reporter.Module,
		fx.NopLogger,
		fx.Invoke(func(*reporter.Reporter) {}),
	)
	app.Start(todo) // nolint: errcheck
	defer func() {
		app.Stop(todo) // nolint: errcheck
	}()

	conn, _ := net.Dial("udp", "127.0.0.1:33811")
	conn.Write([]byte{0x09}) // nolint: errcheck

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	assert.Equal(t, []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00}, buf[:n])
}
