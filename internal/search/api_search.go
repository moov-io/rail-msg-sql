package search

import (
	"net/http"

	"github.com/moov-io/base/log"

	"github.com/gorilla/mux"
)

type Controller interface {
	AppendRoutes(router *mux.Router)
}

func NewController(logger log.Logger, service Service) Controller {
	return &controller{
		logger:  logger,
		service: service,
	}
}

type controller struct {
	logger  log.Logger
	service Service
}

func (c *controller) AppendRoutes(router *mux.Router) {
	router.
		Methods("GET").
		Path("/search").
		HandlerFunc(c.search)
}

func (c *controller) search(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
