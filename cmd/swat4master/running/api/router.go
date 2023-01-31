package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/sergeii/swat4master/api/docs" // nolint: revive
	h "github.com/sergeii/swat4master/cmd/swat4master/running/api/handlers"
	"github.com/sergeii/swat4master/internal/rest/api"
)

func NewRouter(a *api.API) *gin.Engine {
	router := gin.Default()
	router.GET("/status", h.Status)
	router.GET("/api/servers", a.ListServers)
	router.GET("/api/servers/:address", a.ViewServer)
	router.POST("/api/servers", a.AddServer)
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.NewHandler()))
	return router
}
