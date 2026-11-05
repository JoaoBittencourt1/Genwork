package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

// RunServe serves requests while it registers with core in the background; a rejected registration stops the module.
func RunServe(ctx context.Context, getenv platform.Getenv, schema string) error {
	config, err := platform.LoadServeConfig(getenv)
	if err != nil {
		return err
	}
	moduleManifest, err := manifest.ParseManifest(genagents.ManifestYAML, Version)
	if err != nil {
		return err
	}
	db, err := platform.OpenDatabase(ctx, config.DatabaseURL, schema)
	if err != nil {
		return err
	}
	defer db.Close()

	dependencies := app.Dependencies{
		DB: db,
		TokenVerifier: tokens.NewTokenVerifier(tokens.TokenVerifierConfig{
			JWKSURL: config.CoreInternalURL + "/api/core/v1/auth/jwks", Issuer: config.PublicURL,
		}),
		EncryptionKey: config.EncryptionKey,
		Core:          corelink.NewClient(config.CoreInternalURL, corelink.DefaultTimeout),
		Gentasks:      gentasksclient.NewClient(config.GatewayInternalURL, gentasksclient.DefaultTimeout),
		GenworkMCPURL: config.PublicURL + "/mcp",
		Now:           time.Now,
	}
	handler, err := app.New(dependencies)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              config.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serveCtx, stopServing := context.WithCancel(ctx)
	defer stopServing()
	failures := make(chan error, 2)
	workers, err := app.NewWorkers(dependencies)
	if err != nil {
		return err
	}
	for _, worker := range workers {
		go worker.Run(serveCtx)
	}
	go func() {
		slog.Info("genagents listening", "addr", config.HTTPAddr, "version", moduleManifest.Version)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			failures <- fmt.Errorf("http server: %w", err)
		}
	}()
	go func() {
		client := registration.NewRegistrationClient(registration.RegistrationClientConfig{
			CoreURL: config.CoreInternalURL, ModuleSecret: config.ModuleSecret,
		})
		if _, err := client.RegisterUntilAccepted(serveCtx, moduleManifest); err != nil && serveCtx.Err() == nil {
			failures <- fmt.Errorf("register with core: %w", err)
			return
		}
		slog.Info("registered with core", "module", moduleManifest.ID, "version", moduleManifest.Version)
	}()

	var failure error
	select {
	case failure = <-failures:
	case <-ctx.Done():
	}
	slog.Info("shutting down genagents")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return errors.Join(failure, server.Shutdown(shutdownCtx))
}
