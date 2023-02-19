package testutils

import (
	"context"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	"github.com/sergeii/swat4master/cmd/swat4master/modules/api"
)

func PrepareTestServer(tb fxtest.TB, extra ...fx.Option) (*httptest.Server, func()) {
	gin.SetMode(gin.ReleaseMode) // prevent gin from overwriting middlewares

	var router *gin.Engine
	fxopts := []fx.Option{
		fx.Provide(func() config.Config {
			return config.Config{
				HTTPListenAddr: "localhost:11337",
			}
		}),
		application.Module,
		api.Module,
		fx.Decorate(func() *zerolog.Logger {
			logger := zerolog.Nop()
			return &logger
		}),
		fx.NopLogger,
		fx.Populate(&router),
	}
	fxopts = append(fxopts, extra...)

	app := fxtest.New(tb, fxopts...)
	app.RequireStart().RequireStop()

	ts := httptest.NewServer(router)

	return ts, func() {
		defer app.Stop(context.TODO()) // nolint: errcheck
		defer ts.Close()
	}
}
