package testutils

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"

	bootstrap "github.com/sergeii/swat4master/cmd/swat4master/application"
	"github.com/sergeii/swat4master/cmd/swat4master/config"
	running "github.com/sergeii/swat4master/cmd/swat4master/running/api"
	"github.com/sergeii/swat4master/internal/application"
	"github.com/sergeii/swat4master/internal/rest/api"
)

type TestServerOpt func(*config.Config)

func PrepareTestServer(opts ...TestServerOpt) (*httptest.Server, *application.App, func()) {
	cfg := config.Config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	app := bootstrap.Configure()
	gin.SetMode(gin.ReleaseMode) // prevent gin from overwriting middlewares
	rest := api.New(app, cfg)
	ts := httptest.NewServer(running.NewRouter(rest))
	return ts, app, func() {
		defer ts.Close()
	}
}
