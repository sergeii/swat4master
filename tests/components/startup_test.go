package components_test

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/components/api"
	"github.com/sergeii/swat4master/cmd/swat4master/components/exporter"
	"github.com/sergeii/swat4master/tests/testapp"
)

func TestReporter_StartFailsOnBusyPort(t *testing.T) {
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:33811")
	require.NoError(t, err)
	busy, err := net.ListenUDP("udp", udpAddr)
	require.NoError(t, err)
	defer busy.Close()

	app, cancel := makeAppWithReporter()
	defer cancel()

	require.ErrorIs(t, startApp(app), syscall.EADDRINUSE)
}

func TestBrowser_StartFailsOnBusyPort(t *testing.T) {
	occupyTCP(t, "localhost:13382")

	app, cancel := makeAppWithBrowser()
	defer cancel()

	require.ErrorIs(t, startApp(app), syscall.EADDRINUSE)
}

func TestExporter_StartFailsOnBusyPort(t *testing.T) {
	occupyTCP(t, "localhost:11338")

	app := fx.New(
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Supply(exporter.Config{
			HTTPListenAddress: "localhost:11338",
		}),
		exporter.Module,
		fx.NopLogger,
		fx.Invoke(func(*exporter.Component) {}),
	)
	defer app.Stop(context.TODO()) //nolint: errcheck

	require.ErrorIs(t, startApp(app), syscall.EADDRINUSE)
}

func TestAPI_StartFailsOnBusyPort(t *testing.T) {
	occupyTCP(t, "localhost:11339")

	app := fx.New(
		fx.Provide(testapp.NoLogging),
		fx.Provide(testapp.ProvideSettings),
		fx.Provide(testapp.ProvidePersistence),
		application.Module,
		fx.Supply(api.Config{
			HTTPListenAddr: "localhost:11339",
		}),
		api.Module,
		fx.NopLogger,
		fx.Invoke(func(*api.Component) {}),
	)
	defer app.Stop(context.TODO()) //nolint: errcheck

	require.ErrorIs(t, startApp(app), syscall.EADDRINUSE)
}

// startApp starts the app with a deadline,
// so that a component waiting for its server to become ready fails the test instead of hanging it.
func startApp(app *fx.App) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return app.Start(ctx)
}

// occupyTCP listens on the address resolved the same way the servers resolve it,
// so that a server fails to listen on it.
func occupyTCP(t *testing.T, addr string) {
	t.Helper()
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	require.NoError(t, err)
	busy, err := net.ListenTCP("tcp", tcpAddr)
	require.NoError(t, err)
	t.Cleanup(func() {
		busy.Close()
	})
}
