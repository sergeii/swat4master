package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
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
	address, ok := parseViewServerAddress(c.Param("address"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server address"})
		return
	}

	svr, err := a.app.ServerService.Get(c, address)
	if err != nil {
		switch {
		case errors.Is(err, servers.ErrServerNotFound):
			log.Debug().
				Stringer("addr", address).
				Msg("Requested server not found")
			c.Status(http.StatusNotFound)
		default:
			log.Warn().
				Err(err).Stringer("addr", address).
				Msg("Unable to obtain server due to error")
			c.Status(http.StatusInternalServerError)
		}
		return
	}

	if !svr.HasDiscoveryStatus(ds.Details) {
		log.Debug().
			Stringer("addr", address).Stringer("status", svr.GetDiscoveryStatus()).
			Msg("Requested server has no details")
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, model.NewServerDetailFromRepo(svr))
}

func parseViewServerAddress(maybeAddress string) (addr.Addr, bool) {
	maybeIP, maybePort, ok := strings.Cut(maybeAddress, ":")
	if !ok || maybeIP == "" || maybePort == "" {
		return addr.Blank, false
	}

	maybePortNumber, err := strconv.Atoi(maybePort)
	if err != nil {
		return addr.Blank, false
	}

	address, err := addr.NewFromString(maybeIP, maybePortNumber)
	if err != nil {
		return addr.Blank, false
	}

	return address, true
}
