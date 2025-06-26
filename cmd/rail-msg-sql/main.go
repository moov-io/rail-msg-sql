// generated-from:978eb7e8497019d58e3ef1a92840745f9415cc1bceb815251e7a716fdeb0d674 DO NOT REMOVE, DO UPDATE

package main

import (
	"os"

	"github.com/moov-io/base/log"
	railmsgsql "github.com/moov-io/rail-msg-sql"
	"github.com/moov-io/rail-msg-sql/internal/search"
	"github.com/moov-io/rail-msg-sql/internal/storage"
	"github.com/moov-io/rail-msg-sql/pkg/service"
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

	searchController := search.NewController(env.Logger, searchService)
	searchController.AppendRoutes(env.PublicRouter)

	stopServers := env.RunServers(termListener)
	defer stopServers()

	service.AwaitTermination(env.Logger, termListener)
}
