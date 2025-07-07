// generated-from:978eb7e8497019d58e3ef1a92840745f9415cc1bceb815251e7a716fdeb0d674 DO NOT REMOVE, DO UPDATE

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/moov-io/ach-web-viewer/pkg/filelist"
	"github.com/moov-io/base/log"
	"github.com/moov-io/base/telemetry"
	railmsgsql "github.com/moov-io/rail-msg-sql"
	"github.com/moov-io/rail-msg-sql/internal/search"
	"github.com/moov-io/rail-msg-sql/internal/storage"
	"github.com/moov-io/rail-msg-sql/pkg/service"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	env := &service.Environment{
		Logger: log.NewDefaultLogger().Set("app", log.String("rail-msg-sql")).Set("version", log.String(railmsgsql.Version)),
	}

	env, err := service.NewEnvironment(env)
	if err != nil {
		env.Logger.Fatal().LogErrorf("Error loading up environment: %v", err)
		os.Exit(1)
	}
	defer env.Shutdown()

	termListener := service.NewTerminationListener()

	// File Storage
	fileStorage, err := storage.NewRepository(env.Config.Storage)
	if err != nil {
		env.Logger.Fatal().LogErrorf("problem initializing storage repository: %v", err)
		os.Exit(1)
	}

	// Setup search service
	searchService, err := search.NewService(env.Logger, env.Config.Search, fileStorage)
	if err != nil {
		env.Logger.Fatal().LogErrorf("problem initializing search service: %v", err)
		os.Exit(1)
	}

	// Grab the latest files on startup
	if env.Config.Search.BackgroundPrepare {
		go func() {
			err := ingestCurrentFiles(searchService)
			if err != nil {
				env.Logger.Fatal().LogErrorf("problem ingesting current files in the background: %v", err)
				os.Exit(1)
			}
		}()
	}

	searchController := search.NewController(env.Logger, searchService, env.Config.Servers.Public.BasePath)
	searchController.AppendRoutes(env.PublicRouter)

	stopServers := env.RunServers(termListener)
	defer stopServers()

	service.AwaitTermination(env.Logger, termListener)
}

func ingestCurrentFiles(searchService search.Service) error {
	now := time.Now().UTC()
	opts := filelist.DefaultListOptions(now)

	params := storage.FilterParams{
		StartDate: opts.StartDate,
		EndDate:   opts.EndDate,
	}

	ctx, span := telemetry.StartSpan(context.Background(), "background-file-ingest", trace.WithAttributes(
		attribute.String("ingest.start_date", params.StartDate.Format(time.RFC3339)),
		attribute.String("ingest.end_date", params.EndDate.Format(time.RFC3339)),
	))
	defer span.End()

	err := searchService.IngestACHFiles(ctx, params)
	if err != nil {
		return fmt.Errorf("ingesting ach files: %w", err)
	}
	return nil
}
