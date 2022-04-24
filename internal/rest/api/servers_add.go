package api

import "github.com/gin-gonic/gin"

// AddServer godoc
// @Summary      Add server
// @Description  Submit a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        account body      model.AddServer  true  "Server address"
// @Success      200     {object}  model.Server
// @Router       /servers [post]
func (a *API) AddServer(c *gin.Context) {

}
