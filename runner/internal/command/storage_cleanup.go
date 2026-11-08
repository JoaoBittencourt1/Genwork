package command

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/workspace"
)

// idleWorkspaceAge is how long a workspace goes untouched before a cleanup removes it: far longer than any run lasts.
const idleWorkspaceAge = 24 * time.Hour

// storageCleanup frees what the runner no longer needs: sandbox images of other versions and idle workspaces.
type storageCleanup struct {
	launcher   sandbox.Launcher
	workspaces *workspace.Manager
	settings   config.Config
}

func newStorageCleanup(launcher sandbox.Launcher, workspaces *workspace.Manager, settings config.Config) storageCleanup {
	return storageCleanup{launcher: launcher, workspaces: workspaces, settings: settings}
}

// removeOutdatedImages leaves a custom image's repository alone: its other tags belong to whoever built them.
func (cleanup storageCleanup) removeOutdatedImages(ctx context.Context) (int64, error) {
	pruner, ok := cleanup.launcher.(sandbox.ImagePruner)
	if !ok || cleanup.settings.CustomImage || cleanup.settings.SandboxImage == "" {
		return 0, nil
	}
	return pruner.RemoveOutdatedImages(ctx, cleanup.settings.SandboxImage)
}

func (cleanup storageCleanup) run(ctx context.Context) (int64, error) {
	freedImages, err := cleanup.removeOutdatedImages(ctx)
	if err != nil {
		return freedImages, err
	}
	freedWorkspaces, err := cleanup.workspaces.RemoveIdle(idleWorkspaceAge, time.Now())
	return freedImages + freedWorkspaces, err
}
