package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New() *mux.Router {
	router := mux.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	router.Handle("/status", http.HandlerFunc(Status))
	return router
}
