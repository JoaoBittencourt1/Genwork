package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/app"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

func runServe(ctx context.Context, getenv platform.Getenv) error {
	config, err := platform.LoadServeConfig(getenv)
	if err != nil {
		return err
	}
	signingKey, err := auth.ParseSigningKey(config.JWTSigningKey)
	if err != nil {
		return err
	}
	db, err := platform.OpenDatabase(ctx, config.DatabaseURL, platform.CoreSchema)
	if err != nil {
		return err
	}
	defer db.Close()

	handler := app.New(app.Settings{
		Version:        version,
		PublicURL:      config.PublicURL,
		InstanceName:   config.InstanceName,
		DefaultLocale:  config.DefaultLocale,
		SigningKey:     signingKey,
		TrustedProxies: platform.NewClientAddressResolver(config.TrustedProxies),
		PasswordParams: users.DefaultPasswordParams,
		RateLimits:     app.DefaultRateLimits,
	}, app.Dependencies{DB: db, Mailer: mail.NewSMTPSender(config.SMTP), Now: time.Now})

	server := &http.Server{
		Addr:              config.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("core listening", "addr", config.HTTPAddr, "version", version)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down core")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
