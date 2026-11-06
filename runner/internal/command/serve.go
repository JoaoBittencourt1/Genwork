package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/claims"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/execution"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/github"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/heartbeat"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/workspace"
)

// ServeOptions lets tests replace the sandbox and shorten the intervals; the zero value is the real runner.
type ServeOptions struct {
	Launcher          sandbox.Launcher
	HeartbeatInterval time.Duration
	PollInterval      time.Duration
	DirectoryMode     bool
}

func runServe(ctx context.Context, args []string, getenv config.Getenv, options ServeOptions) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	directoryMode := flags.Bool("directory-mode", false, "run CLIs on the host without a sandbox (development only)")
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	options.DirectoryMode = options.DirectoryMode || *directoryMode
	return Serve(ctx, getenv, options)
}

// Serve claims and executes runs until the context ends or the runner is revoked.
func Serve(ctx context.Context, getenv config.Getenv, options ServeOptions) error {
	settings, err := config.Load(getenv, Version, getenv("GENWORK_IMAGE_REGISTRY"))
	if err != nil {
		return err
	}
	launcher, err := chooseLauncher(settings, getenv, options)
	if err != nil {
		return err
	}
	if settings.SandboxImage == "" && !options.DirectoryMode {
		return errors.New("set GENWORK_IMAGE_REGISTRY, or GENWORK_SANDBOX_IMAGE for a custom image: the runner needs a sandbox image")
	}
	workspaces := workspace.NewManager(settings.WorkspacesDir(), settings.GitBaseURL, settings.GithubToken)
	client := api.NewClient(settings.InstanceURL, settings.RunnerSecret, nil)
	if err := recoverLeftovers(ctx, launcher, workspaces, client); err != nil {
		return err
	}
	executor := execution.NewExecutor(client, launcher, workspaces,
		execution.NewGithubAdapter(github.NewClient(settings.GithubAPIURL, settings.GithubToken, &http.Client{Timeout: 30 * time.Second})),
		execution.Settings{
			SandboxImage: settings.SandboxImage, Limits: sandbox.Limits{CPUs: settings.Limits.CPUs, Memory: settings.Limits.Memory, PIDs: settings.Limits.PIDs},
			ProfileDir: settings.ProfileDir, PollInterval: options.PollInterval,
		})
	loop := claims.NewLoop(client, executor, settings.MaxConcurrency)
	beat := heartbeat.NewLoop(client, launcher, heartbeat.Settings{
		Image: settings.SandboxImage, CustomImage: settings.CustomImage, HasGithubCredentials: settings.HasGithubCredentials(),
		MaxConcurrency: settings.MaxConcurrency, ProfilesDir: settings.ProfilesDir(), Interval: options.HeartbeatInterval,
	})
	return runLoops(ctx, loop, beat)
}

func chooseLauncher(settings config.Config, getenv config.Getenv, options ServeOptions) (sandbox.Launcher, error) {
	switch {
	case options.Launcher != nil:
		return options.Launcher, nil
	case options.DirectoryMode:
		slog.Warn("directory mode: CLIs run on this machine without isolation; use it for development only")
		return sandbox.NewDirectorySandbox(), nil
	}
	socket, err := sandbox.FindSocket(settings.EngineSocket, getenv)
	if err != nil {
		return nil, err
	}
	return sandbox.NewContainerSandbox(socket), nil
}

// recoverLeftovers removes the sandboxes and workspaces a crash left behind, and tells genagents those runs were lost.
func recoverLeftovers(ctx context.Context, launcher sandbox.Launcher, workspaces *workspace.Manager, client *api.Client) error {
	removed, err := launcher.RemoveLeftovers(ctx, nil)
	if err != nil && !errors.Is(err, sandbox.ErrNoEngine) {
		return err
	}
	if err := workspaces.RemoveAll(nil); err != nil {
		return err
	}
	for _, runToken := range removed {
		payload := []byte(`{"reason":"runner_lost","message":"the runner restarted while the run was executing"}`)
		if _, err := client.PostEvents(ctx, runToken, []api.Event{{Sequence: 1 << 40, Type: "error", Payload: payload}}); err != nil {
			slog.Warn("report a lost run", "run", runToken, "err", err)
		}
	}
	return nil
}

// runLoops runs the heartbeat and the claim loop together; either one ending stops the other.
func runLoops(ctx context.Context, loop *claims.Loop, beat *heartbeat.Loop) error {
	loopCtx, stop := context.WithCancel(ctx)
	defer stop()
	failures := make(chan error, 2)
	go func() { failures <- beat.Run(loopCtx) }()
	go func() { failures <- loop.Run(loopCtx) }()
	first := <-failures
	stop()
	second := <-failures
	if errors.Is(first, claims.ErrRevoked) || errors.Is(second, claims.ErrRevoked) {
		return claims.ErrRevoked
	}
	return errors.Join(first, second)
}
