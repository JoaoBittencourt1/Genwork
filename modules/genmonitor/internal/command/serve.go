package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

// RunServe serves requests while it registers with core in the background; a rejected registration stops the module.
func RunServe(ctx context.Context, getenv platform.Getenv) error {
	config, err := platform.LoadServeConfig(getenv)
	if err != nil {
		return err
	}
	moduleManifest, err := manifest.ParseManifest(genmonitor.ManifestYAML)
	if err != nil {
		return err
	}

	serveCtx, stopServing := context.WithCancel(ctx)
	defer stopServing()
	sampler := hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{
		Reader:           hostmetrics.NewProcHostReader(config.HostProcPath, config.HostRootPath, time.Now),
		Interval:         hostmetrics.SampleInterval,
		FirstSampleDelay: hostmetrics.FirstSampleDelay,
	})
	if err := sampler.Start(serveCtx); err != nil {
		return fmt.Errorf("sample host: %w", err)
	}
	server := &http.Server{
		Addr: config.HTTPAddr,
		Handler: app.New(app.Dependencies{
			TokenVerifier: tokens.NewTokenVerifier(tokens.TokenVerifierConfig{
				JWKSURL: config.CoreInternalURL + "/api/core/v1/auth/jwks", Issuer: config.PublicURL,
			}),
			HostSnapshots: sampler,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	failures := make(chan error, 2)
	go func() {
		slog.Info("genmonitor listening", "addr", config.HTTPAddr, "version", moduleManifest.Version)
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
	slog.Info("shutting down genmonitor")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return errors.Join(failure, server.Shutdown(shutdownCtx))
}
