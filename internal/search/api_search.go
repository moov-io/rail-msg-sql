package search

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/ach-web-viewer/pkg/filelist"
	"github.com/moov-io/base/log"
	"github.com/moov-io/base/telemetry"
	"github.com/moov-io/rail-msg-sql/internal/storage"
	"github.com/moov-io/rail-msg-sql/webui"
	"go.opentelemetry.io/otel/attribute"

	"github.com/gorilla/mux"
)

type Controller interface {
	AppendRoutes(router *mux.Router)
}

func NewController(logger log.Logger, service Service, basePath string) Controller {
	return &controller{
		logger:   logger,
		service:  service,
		basePath: basePath,
	}
}

type controller struct {
	logger   log.Logger
	service  Service
	basePath string
}

func (c *controller) AppendRoutes(router *mux.Router) {
	// Static CSS and JS
	staticFS := http.FileServer(http.FS(webui.WebRoot))
	router.PathPrefix("/static/").Handler(http.StripPrefix(c.basePath, staticFS))

	router.
		Methods("GET").
		Path("/").
		HandlerFunc(c.index)

	router.
		Methods("POST").
		Path("/search").
		HandlerFunc(c.search)
}

func (c *controller) index(w http.ResponseWriter, r *http.Request) {
	opts, err := filelist.ReadListOptions(r)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("index: reading list options: %w", err))
		return
	}
	w.Header().Set("Content-Type", "text/html")

	data := indexData{
		BaseURL:      baseURL(c.basePath),
		TimeRangeMin: opts.StartDate,
		TimeRangeMax: opts.EndDate,
	}

	err = indexTemplate.Execute(w, data)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("ERROR: rendering index: %w", err))
		return
	}
}

type searchRequest struct {
	Query string `json:"query"`
}

func (c *controller) search(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.StartSpan(r.Context(), "api-search")
	defer span.End()

	var req searchRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("reading search request: %v", err))
		return
	}
	query, err := base64.StdEncoding.DecodeString(req.Query)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("problem decoding query body: %v", err))
		return
	}

	options, err := filelist.ReadListOptions(r)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("problem reading search request params: %v", err))
		return
	}

	params := storage.FilterParams{
		StartDate: options.StartDate,
		EndDate:   options.EndDate,
		Pattern:   options.Pattern,
	}
	span.SetAttributes(
		attribute.String("search.start_date", params.StartDate.Format(time.RFC3339)),
		attribute.String("search.end_date", params.EndDate.Format(time.RFC3339)),
		attribute.String("search.pattern", params.Pattern),
	)

	fmt.Printf("search: %#v\n", params)

	err = c.service.IngestACHFiles(ctx, params)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("problem ingesting files for search: %v", err))
		return
	}

	results, err := c.service.Search(ctx, string(query), params)
	if err != nil {
		c.errorResponse(w, fmt.Errorf("problem reading search request params: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(results)
}

type errorResponse struct {
	Error string `json:"error"`
}

func (c *controller) errorResponse(w http.ResponseWriter, err error) {
	message := c.logger.Error().LogError(err).Err()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	json.NewEncoder(w).Encode(errorResponse{
		Error: message.Error(),
	})
	return
}
