// Package heartbeat tells genagents every few seconds that the runner is alive, what it can run, and what it holds.
package heartbeat

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/claims"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const Interval = 15 * time.Second

type Sender interface {
	SendHeartbeat(ctx context.Context, heartbeat api.Heartbeat) (api.HeartbeatAnswer, error)
}

type Settings struct {
	Image                string
	CustomImage          bool
	HasGithubCredentials bool
	MaxConcurrency       int
	ProfilesDir          string
	Interval             time.Duration
}

type Loop struct {
	sender   Sender
	launcher sandbox.Launcher
	settings Settings
}

func NewLoop(sender Sender, launcher sandbox.Launcher, settings Settings) *Loop {
	if settings.Interval == 0 {
		settings.Interval = Interval
	}
	return &Loop{sender: sender, launcher: launcher, settings: settings}
}

// Beat sends one heartbeat and deletes the sign-in profile of every account genagents no longer lists for this runner.
func (loop *Loop) Beat(ctx context.Context) error {
	info, err := loop.launcher.Describe(ctx, loop.settings.Image)
	if err != nil {
		return err
	}
	answer, err := loop.sender.SendHeartbeat(ctx, api.Heartbeat{
		ContainerEngine: info.Engine, SandboxImage: loop.settings.Image, CustomImage: loop.settings.CustomImage, DirectoryMode: info.DirectoryMode,
		Tools: info.Tools, HasGithubCredentials: loop.settings.HasGithubCredentials, MaxConcurrency: loop.settings.MaxConcurrency,
	})
	if err != nil {
		return err
	}
	return loop.removeStaleProfiles(answer.Accounts)
}

func (loop *Loop) removeStaleProfiles(assigned []api.AccountAssignment) error {
	entries, err := os.ReadDir(loop.settings.ProfilesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	held := make(map[string]bool, len(assigned))
	for _, account := range assigned {
		held[account.Token] = true
	}
	for _, entry := range entries {
		if !held[entry.Name()] {
			if err := os.RemoveAll(loop.settings.ProfilesDir + "/" + entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run beats until the context ends, or the runner is revoked.
func (loop *Loop) Run(ctx context.Context) error {
	ticker := time.NewTicker(loop.settings.Interval)
	defer ticker.Stop()
	for {
		err := loop.Beat(ctx)
		var statusErr *api.StatusError
		switch {
		case errors.As(err, &statusErr) && statusErr.IsRevoked():
			return claims.ErrRevoked
		case err != nil && ctx.Err() == nil:
			slog.Warn("send a heartbeat", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
