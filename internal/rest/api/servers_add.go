package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/sergeii/swat4master/internal/core/probes"
	repo "github.com/sergeii/swat4master/internal/core/servers"
	"github.com/sergeii/swat4master/internal/entity/addr"
	ds "github.com/sergeii/swat4master/internal/entity/discovery/status"
	"github.com/sergeii/swat4master/internal/rest/model"
	"github.com/sergeii/swat4master/internal/services/discovery/finding"
	"github.com/sergeii/swat4master/internal/services/server"
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
	address, err := parseAddServerAddress(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error{
			"invalid server address",
		})
		return
	}

	svr, err := a.app.ServerService.Get(c, address)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrServerNotFound):
			newSvr, err := createServerFromAddress(c, address, a.app.ServerService)
			if err != nil {
				log.Error().
					Err(err).Stringer("addr", address).
					Msg("Unable to create new server")
				c.Status(http.StatusInternalServerError)
				return
			}
			a.addServer(c, newSvr)
		default:
			log.Warn().
				Err(err).Stringer("addr", address).
				Msg("Unable to obtain server due to error")
			c.Status(http.StatusInternalServerError)
		}
		return
	}

	a.addServer(c, svr)
}

func (a *API) addServer(c *gin.Context, svr repo.Server) {
	switch {
	case svr.HasDiscoveryStatus(ds.Details):
		log.Debug().
			Stringer("server", svr).Stringer("status", svr.GetDiscoveryStatus()).
			Msg("Server already has details")
		c.JSON(http.StatusOK, model.NewServerFromRepo(svr))
	// server discovery is still pending
	case svr.HasAnyDiscoveryStatus(ds.PortRetry | ds.DetailsRetry):
		log.Debug().
			Stringer("server", svr).Stringer("status", svr.GetDiscoveryStatus()).
			Msg("Server discovery is in progress")
		c.Status(http.StatusAccepted)
	case svr.HasDiscoveryStatus(ds.NoPort):
		log.Debug().
			Stringer("server", svr).Stringer("status", svr.GetDiscoveryStatus()).
			Msg("No port has been discovered for server")
		c.Status(http.StatusGone)
	// other status - send the server for port discovery
	default:
		if discErr := discoverServer(c, a.app.ServerService, a.app.FindingService, svr); discErr != nil {
			log.Warn().
				Err(discErr).Stringer("server", svr).
				Msg("Unable to submit discovery for server")
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusAccepted)
	}
}

func parseAddServerAddress(c *gin.Context) (addr.Addr, error) {
	var req model.NewServer

	if err := c.ShouldBindJSON(&req); err != nil {
		return addr.Blank, err
	}

	address, err := addr.NewFromString(req.IP, req.Port)
	if err != nil {
		return addr.Blank, err
	}

	return address, nil
}

func createServerFromAddress(
	ctx context.Context,
	address addr.Addr,
	servers *server.Service,
) (repo.Server, error) {
	svr, err := repo.NewFromAddr(address, address.Port+1)
	if err != nil {
		return repo.Blank, err
	}

	if svr, err = servers.Create(ctx, svr, func(upd *repo.Server) bool {
		// a server with exactly same address was created in the process,
		// we cannot proceed further
		return false
	}); err != nil {
		return repo.Blank, err
	}

	return svr, nil
}

func discoverServer(
	ctx context.Context,
	servers *server.Service,
	finder *finding.Service,
	svr repo.Server,
) error {
	var err error

	svr.UpdateDiscoveryStatus(ds.PortRetry)

	if svr, err = servers.Update(ctx, svr, func(updated *repo.Server) bool {
		// some of the statuses that we don't want to run discovery for has appeared
		// in the process, so we abort here
		if svr.HasDiscoveryStatus(ds.Details | ds.PortRetry | ds.DetailsRetry) {
			return false
		}
		// prevent further submissions until the retry status is cleared
		updated.UpdateDiscoveryStatus(ds.PortRetry)
		return true
	}); err != nil {
		return err
	}

	if err = finder.DiscoverPort(ctx, svr.GetAddr(), probes.NC, probes.NC); err != nil {
		return err
	}

	log.Info().Stringer("server", svr).Msg("Added existing server for rediscovery")

	return nil
}
