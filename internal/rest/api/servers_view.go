package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/usecases/getserver"
	"github.com/sergeii/swat4master/internal/rest/model"
)

// ViewServer godoc
// @Summary      View server detail
// @Description  Return detailed information for a specific server
// @Tags         servers
// @Produce      json
// @Success      200 {object} model.ServerDetail
// @Router       /servers/:address [get]
func (a *API) ViewServer(c *gin.Context) {
	address, err := addr.NewFromString(c.Param("address"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server address"})
		return
	}

	svr, err := a.container.GetServer.Execute(c, address)
	if err != nil {
		switch {
		case errors.Is(err, getserver.ErrServerNotFound):
			a.logger.Debug().
				Stringer("addr", address).
				Msg("Requested server not found")
			c.Status(http.StatusNotFound)
		case errors.Is(err, getserver.ErrServerHasNoDetails):
			a.logger.Debug().
				Stringer("addr", address).
				Msg("Requested server has no details")
			c.Status(http.StatusNoContent)
		}
		return
	}

	c.JSON(http.StatusOK, model.NewServerDetailFromDomain(svr))
}
