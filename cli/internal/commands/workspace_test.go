package commands

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindWorkspaceWalksUpFromSubdirectories(t *testing.T) {
	workspace := newTestWorkspace(t)
	found, err := FindWorkspace(filepath.Join(workspace.Root, "web"))
	if err != nil || found.Root != workspace.Root {
		t.Fatalf("FindWorkspace = %+v, %v", found, err)
	}
	if _, err := FindWorkspace(t.TempDir()); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestEnsureEnvFilesCreatesMissingFilesWithSigningKey(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatalf("EnsureEnvFiles: %v", err)
	}
	seed, err := base64.StdEncoding.DecodeString(workspace.StackEnvValue("JWT_SIGNING_KEY"))
	if err != nil || len(seed) != 32 {
		t.Fatalf("signing key is not a 32-byte base64 seed: %q", workspace.StackEnvValue("JWT_SIGNING_KEY"))
	}
	if workspace.StackEnvValue("CORE_PORT") != "8484" || workspace.WebEnvValue("DEV_SERVER_PORT") != "8485" {
		t.Error("example values must be copied")
	}
	info, _ := os.Stat(workspace.StackEnvFile())
	if info.Mode().Perm() != 0o600 {
		t.Errorf("stack .env permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestEnsureEnvFilesNeverOverwrites(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := os.WriteFile(workspace.StackEnvFile(), []byte("JWT_SIGNING_KEY=mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if workspace.StackEnvValue("JWT_SIGNING_KEY") != "mine" {
		t.Error("an existing .env must be left untouched")
	}
}
