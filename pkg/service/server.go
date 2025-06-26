// generated-from:dacae6e9fd93338b042ab15d1352dd3c05ada5b4fdf3b384911cead7c7fcda3d DO NOT REMOVE, DO UPDATE

package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/moov-io/base/admin"
	"github.com/moov-io/base/log"

	_ "github.com/moov-io/rail-msg-sql"
)

// RunServers - Boots up all the servers and awaits till they are stopped.
func (env *Environment) RunServers(terminationListener chan error) func() {

	adminServer := bootAdminServer(terminationListener, env.Logger, env.Config.Servers.Admin)

	_, shutdownPublicServer := bootHTTPServer("public", env.PublicRouter, terminationListener, env.Logger, env.Config.Servers.Public)

	return func() {
		adminServer.Shutdown()
		shutdownPublicServer()
	}
}

func bootHTTPServer(name string, routes *mux.Router, errs chan<- error, logger log.Logger, config HTTPConfig) (*http.Server, func()) {
	// Create main HTTP server
	serve := &http.Server{
		Addr:    config.Bind.Address,
		Handler: routes,
		TLSConfig: &tls.Config{
			InsecureSkipVerify:       false,
			PreferServerCipherSuites: true,
			MinVersion:               tls.VersionTLS12,
		},
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start main HTTP server
	go func() {
		logger.Info().Log(fmt.Sprintf("%s listening on %s", name, config.Bind.Address))
		if err := serve.ListenAndServe(); err != nil {
			errs <- logger.Fatal().LogErrorf("problem starting http: %w", err).Err()
		}
	}()

	shutdownServer := func() {
		if err := serve.Shutdown(context.Background()); err != nil {
			logger.Fatal().LogErrorf("shutting down: %v", err)
		}
	}

	return serve, shutdownServer
}

func bootAdminServer(errs chan<- error, logger log.Logger, config HTTPConfig) *admin.Server {
	adminServer, err := admin.New(admin.Opts{
		Addr: config.Bind.Address,
	})
	if err != nil {
		errs <- logger.Fatal().LogErrorf("running admin server: %w", err).Err()
		return nil
	}

	go func() {
		logger.Info().Log(fmt.Sprintf("listening on %s", adminServer.BindAddr()))
		if err := adminServer.Listen(); err != nil {
			errs <- logger.Fatal().LogErrorf("problem starting admin http: %w", err).Err()
		}
	}()

	return adminServer
}
