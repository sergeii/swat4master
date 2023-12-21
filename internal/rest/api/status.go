package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
)

func (a *API) Status(c *gin.Context) {
	status := map[string]string{
		"BuildTime":    build.Time,
		"BuildCommit":  build.Commit,
		"BuildVersion": build.Version,
	}
	c.JSON(http.StatusOK, status)
}
