package reporter_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/running"
	"github.com/sergeii/swat4master/cmd/swat4master/running/reporter"
)

func TestReporter_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cfg := config.Config{
		ReporterListenAddr: "127.0.0.1:33811",
		ReporterBufferSize: 1024,
	}
	app := application.Configure()
	runner := running.NewRunner(app, cfg)
	runner.Add(reporter.Run, ctx)
	runner.WaitReady()

	conn, _ := net.Dial("udp", "127.0.0.1:33811")
	conn.Write([]byte{0x09}) // nolint: errcheck

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	assert.Equal(t, buf[:n], []byte{0xfe, 0xfd, 0x09, 0x00, 0x00, 0x00, 0x00})

	cancel()
	runner.WaitQuit()
}
