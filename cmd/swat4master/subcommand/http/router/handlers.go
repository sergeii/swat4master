package router

import (
	"net/http"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
	"github.com/sergeii/swat4master/pkg/http/resp"
)

func Status(rw http.ResponseWriter, r *http.Request) {
	status := map[string]string{
		"BuildTime":    build.Time,
		"BuildCommit":  build.Commit,
		"BuildVersion": build.Version,
	}
	resp.JSONResponse(status, rw, http.StatusOK)
}
