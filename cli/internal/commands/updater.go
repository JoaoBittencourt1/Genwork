package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/updates"
	"github.com/spf13/cobra"
)

const minimumUpdaterSecretLength = 22

type updaterConfig struct {
	address    string
	secret     string
	gatewayURL string
}

// loadUpdaterConfig fails at startup instead of falling back to defaults (rule 1), and never echoes the secret (rule 67).
func loadUpdaterConfig(getenv func(string) string) (updaterConfig, error) {
	config := updaterConfig{address: getenv("UPDATER_HTTP_ADDR"), secret: getenv("GENWORK_UPDATER_SECRET"), gatewayURL: getenv("GATEWAY_INTERNAL_URL")}
	if missing := missingVariables(getenv, "UPDATER_HTTP_ADDR", "GENWORK_UPDATER_SECRET", "GATEWAY_INTERNAL_URL"); len(missing) > 0 {
		return updaterConfig{}, fmt.Errorf("the updater needs %s", strings.Join(missing, ", "))
	}
	if len(config.secret) < minimumUpdaterSecretLength {
		return updaterConfig{}, fmt.Errorf("GENWORK_UPDATER_SECRET must be at least %d characters", minimumUpdaterSecretLength)
	}
	return config, nil
}

// missingVariables lists, in the given order, the variables that are unset or blank.
func missingVariables(getenv func(string) string, names ...string) []string {
	var missing []string
	for _, name := range names {
		if strings.TrimSpace(getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func newUpdaterCommand(dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "updater",
		Short: "Serve the update API core relays to admins; runs in the updater container, the only one with the Docker socket",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runUpdater(command, dependencies)
		},
	}
	command.AddCommand(&cobra.Command{
		Use:   "replace",
		Short: "Recreate the updater container with the installed version",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := locateWorkspace(dependencies)
			if err != nil {
				return err
			}
			installation := newWorkspaceInstallation(dependencies, workspace, command.OutOrStdout(), command.ErrOrStderr(), "")
			version, err := workspace.Version()
			if err != nil {
				return err
			}
			return installation.replaceUpdater(command.Context(), version)
		},
	})
	return command
}

func runUpdater(command *cobra.Command, dependencies Dependencies) error {
	config, err := loadUpdaterConfig(dependencies.lookupEnv)
	if err != nil {
		return err
	}
	workspace, err := locateWorkspace(dependencies)
	if err != nil {
		return err
	}
	channel, err := newReleaseChannel(workspace)
	if err != nil {
		return err
	}
	output := command.ErrOrStderr()
	installation := newWorkspaceInstallation(dependencies, workspace, command.OutOrStdout(), output, config.gatewayURL)
	installation.insideUpdater = true
	runner := updates.NewUpdateRunner(updates.UpdateRunnerConfig{Installation: installation, Channel: channel, Now: time.Now, Progress: output})
	service := updates.NewUpdaterService(runner, func(ctx context.Context, status updates.UpdateStatus) {
		if err := installation.replaceUpdater(ctx, status.InstalledVersion); err != nil {
			slog.Error("replace the updater container", "err", err)
		}
	})
	return serveUpdater(command.Context(), config.address, updates.NewUpdaterHandler(service, config.secret))
}

func serveUpdater(ctx context.Context, address string, handler http.Handler) error {
	server := &http.Server{
		Addr: address, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: time.Minute, IdleTimeout: 2 * time.Minute,
	}
	failures := make(chan error, 1)
	go func() {
		slog.Info("updater listening", "addr", address)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			failures <- err
		}
	}()
	select {
	case err := <-failures:
		return fmt.Errorf("updater: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
