package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/usecases/addserver"
	"github.com/sergeii/swat4master/internal/rest/model"
)

// AddServer godoc
// @Summary      Add server
// @Description  Add a new server by submitting its address
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        server  body      model.NewServer  true  "Server address"
// @Success      200     {object}  model.Server
// @Success      202     "Server address has been submitted"
// @Router       /servers [post]
func (a *API) AddServer(c *gin.Context) {
	address, parseErr := parseAddServerAddress(c)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server address"})
		return
	}

	svr, err := a.container.AddServer.Execute(c, address)
	if err != nil {
		switch {
		case errors.Is(err, addserver.ErrInvalidAddress):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server address"})
		case errors.Is(err, addserver.ErrUnableToCreateServer):
			c.Status(http.StatusInternalServerError)
		case errors.Is(err, addserver.ErrUnableToDiscoverServer):
			c.Status(http.StatusInternalServerError)
		case errors.Is(err, addserver.ErrServerDiscoveryInProgress):
			c.Status(http.StatusAccepted)
		case errors.Is(err, addserver.ErrServerHasNoQueryablePort):
			c.Status(http.StatusGone)
		}
		return
	}

	c.JSON(http.StatusOK, model.NewServerFromDomain(svr))
}

func parseAddServerAddress(c *gin.Context) (addr.PublicAddr, error) {
	var req model.NewServer

	if err := c.ShouldBindJSON(&req); err != nil {
		return addr.BlankPublicAddr, err
	}

	address, err := addr.NewFromDotted(req.IP, req.Port)
	if err != nil {
		return addr.BlankPublicAddr, err
	}

	pubAddress, err := addr.NewPublicAddr(address)
	if err != nil {
		return addr.BlankPublicAddr, err
	}

	return pubAddress, nil
}
