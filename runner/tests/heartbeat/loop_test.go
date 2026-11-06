package heartbeat_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/claims"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/heartbeat"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandboxtest"
)

func newLoop(t *testing.T, directoryMode bool) (*heartbeat.Loop, *genagentstest.FakeGenagents, string) {
	t.Helper()
	fake := genagentstest.NewFakeGenagents(t)
	launcher := sandboxtest.NewFakeLauncher(nil)
	launcher.DescribeAs = sandbox.Info{Engine: "Docker 27", ImageDigest: "sha256:abc", Tools: map[string]string{"claude": "2.1.0", "codex": "0.30.0"}, DirectoryMode: directoryMode}
	profiles := t.TempDir()
	loop := heartbeat.NewLoop(api.NewClient(fake.URL(), genagentstest.Secret, nil), launcher, heartbeat.Settings{
		Image: "registry.example/genwork-sandbox:2.0.0", CustomImage: true, HasGithubCredentials: true, MaxConcurrency: 3, ProfilesDir: profiles, Interval: 20 * time.Millisecond,
	})
	return loop, fake, profiles
}

func TestHeartbeatReportsEngineImageToolsAndCredentialPresence(t *testing.T) {
	loop, fake, _ := newLoop(t, false)

	if err := loop.Beat(context.Background()); err != nil {
		t.Fatalf("beat: %v", err)
	}

	sent := fake.Heartbeats[0]
	tools := sent["tools"].(map[string]any)
	if sent["containerEngine"] != "Docker 27" || sent["sandboxImage"] != "registry.example/genwork-sandbox:2.0.0" || sent["customImage"] != true ||
		sent["directoryMode"] != false || sent["hasGithubCredentials"] != true || sent["maxConcurrency"].(float64) != 3 || tools["claude"] != "2.1.0" || tools["codex"] != "0.30.0" {
		t.Errorf("heartbeat = %v", sent)
	}
	for key, value := range sent {
		if text, ok := value.(string); ok && (text == "ghp-token" || text == genagentstest.Secret) {
			t.Errorf("%s carries a credential", key)
		}
	}
}

func TestHeartbeatReportsDirectoryMode(t *testing.T) {
	loop, fake, _ := newLoop(t, true)

	if err := loop.Beat(context.Background()); err != nil {
		t.Fatalf("beat: %v", err)
	}

	if fake.Heartbeats[0]["directoryMode"] != true {
		t.Errorf("heartbeat = %v", fake.Heartbeats[0])
	}
}

func TestProfilesOfRemovedAccountsAreDeleted(t *testing.T) {
	loop, fake, profiles := newLoop(t, false)
	for _, account := range []string{"kept-account", "removed-account"} {
		if err := os.MkdirAll(filepath.Join(profiles, account), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profiles, account, "auth.json"), []byte("credentials"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fake.AssignAccount("kept-account", "anthropic")

	if err := loop.Beat(context.Background()); err != nil {
		t.Fatalf("beat: %v", err)
	}

	if _, err := os.Stat(filepath.Join(profiles, "kept-account")); err != nil {
		t.Errorf("a held profile was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profiles, "removed-account")); !os.IsNotExist(err) {
		t.Error("the profile of a removed account survived")
	}
}

func TestLoopBeatsRepeatedlyAndStopsWhenRevoked(t *testing.T) {
	loop, fake, _ := newLoop(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	if err := loop.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if fake.HeartbeatCount() < 3 {
		t.Errorf("heartbeats = %d", fake.HeartbeatCount())
	}

	fake.Revoke()
	if err := loop.Run(context.Background()); !errors.Is(err, claims.ErrRevoked) {
		t.Errorf("error = %v", err)
	}
}
