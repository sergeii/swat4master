package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/rest/model"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

type ServerFilterForm struct {
	GameVariant    string `form:"gamevariant"`
	GameVer        string `form:"gamever"`
	GameType       string `form:"gametype"`
	HidePassworded bool   `form:"nopassworded"`
	HideFull       bool   `form:"nofull"`
	HideEmpty      bool   `form:"noempty"`
}

// ListServers godoc
// @Summary      List servers
// @Description  List servers that report to the master server as well as the servers discovered by other means
// @Tags         servers
// @Produce      json
// @Param        gamevariant     query    string  false  "Game variant (SWAT 4, SWAT 4X, etc)"
// @Param        gamever         query    string  false  "Game version (1.0, 1.1, etc)"
// @Param        gametype        query    string  false  "Game mode (VIP Escort, CO-OP, etc)"
// @Param        nopassworded    query    bool    false  "Hide password protected servers"
// @Param        nofull          query    bool    false  "Hide full servers"
// @Param        noempty         query    bool    false  "Hide empty servers"
// @Success      200 {array} model.Server
// @Router       /servers [get]
func (a *API) ListServers(c *gin.Context) {
	var form ServerFilterForm
	if err := c.ShouldBindQuery(&form); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	q := prepareQuery(form)
	ucRequest := listservers.NewRequest(q, a.settings.ServerLiveness, ds.Info)

	servers, err := a.container.ListServers.Execute(c, ucRequest)
	if err != nil {
		a.logger.Err(err).Dur("liveness", a.settings.ServerLiveness).Msg("Failed to obtain servers")
		c.Status(http.StatusInternalServerError)
		return
	}

	result := make([]model.Server, 0, len(servers))
	for _, svr := range servers {
		result = append(result, model.NewServerFromDomain(svr))
	}
	c.JSON(http.StatusOK, result)
}

func maybeAddFilter(filters []filter.Filter, f filter.Filter, err error) []filter.Filter {
	if err == nil {
		filters = append(filters, f)
	}
	return filters
}

func prepareQuery(form ServerFilterForm) query.Query {
	filters := make([]filter.Filter, 0)
	if form.GameVariant != "" {
		f, err := filter.New("gamevariant", "=", form.GameVariant)
		filters = maybeAddFilter(filters, f, err)
	}
	if form.GameVer != "" {
		f, err := filter.New("gamever", "=", form.GameVer)
		filters = maybeAddFilter(filters, f, err)
	}
	if form.GameType != "" {
		f, err := filter.New("gametype", "=", form.GameType)
		filters = maybeAddFilter(filters, f, err)
	}
	if form.HidePassworded {
		f, err := filter.New("password", "!=", 1)
		filters = maybeAddFilter(filters, f, err)
	}
	if form.HideFull {
		f, err := filter.New("numplayers", "!=", filter.NewFieldValue("maxplayers"))
		filters = maybeAddFilter(filters, f, err)
	}
	if form.HideEmpty {
		f, err := filter.New("numplayers", ">", 0)
		filters = maybeAddFilter(filters, f, err)
	}
	if len(filters) > 0 {
		if q, err := query.New(filters); err == nil {
			return q
		}
	}
	return query.Blank
}
