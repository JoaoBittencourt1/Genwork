package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
)

func envFrom(values map[string]string) config.Getenv {
	return func(name string) string { return values[name] }
}

func registered(t *testing.T, extra map[string]string) (string, config.Getenv) {
	t.Helper()
	home := t.TempDir()
	if err := config.Save(home, config.Stored{InstanceURL: "https://genwork.example.com/", RunnerToken: "runner-token", RunnerSecret: "secret"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	values := map[string]string{"GENWORK_RUNNER_HOME": home}
	for key, value := range extra {
		values[key] = value
	}
	return home, envFrom(values)
}

func TestSaveWritesTheSecretWithOwnerOnlyPermissions(t *testing.T) {
	home := filepath.Join(t.TempDir(), "runner")

	if err := config.Save(home, config.Stored{InstanceURL: "https://x.io", RunnerSecret: "secret", GithubToken: "ghp-token"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(filepath.Join(home, "runner.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, err = %v", info.Mode(), err)
	}
	directory, _ := os.Stat(home)
	if directory.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v", directory.Mode())
	}
}

func TestLoadReadsTheFileAndAppliesDefaults(t *testing.T) {
	_, getenv := registered(t, nil)

	loaded, err := config.Load(getenv, "2.0.0", "ghcr.io/acme")

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.InstanceURL != "https://genwork.example.com" || loaded.RunnerSecret != "secret" || loaded.MaxConcurrency != 2 ||
		loaded.SandboxImage != "ghcr.io/acme/genwork-sandbox:2.0.0" || loaded.CustomImage || loaded.GitBaseURL != "https://github.com" ||
		loaded.GithubAPIURL != "https://api.github.com" {
		t.Errorf("config = %+v", loaded)
	}
	if loaded.Limits.CPUs != 4 || loaded.Limits.Memory != 8<<30 || loaded.Limits.PIDs != 1024 {
		t.Errorf("limits = %+v", loaded.Limits)
	}
	if loaded.HasGithubCredentials() {
		t.Error("credentials reported without a token")
	}
}

func TestEnvironmentOverridesTheFile(t *testing.T) {
	_, getenv := registered(t, map[string]string{
		"GENWORK_INSTANCE_URL": "https://other.example.com", "GH_TOKEN": "ghp-env", "GENWORK_SANDBOX_IMAGE": "my/image:1",
		"GENWORK_RUNNER_CONCURRENCY": "5", "GENWORK_GIT_BASE_URL": "file:///srv/git", "GITHUB_API_URL": "http://api.test",
		"GENWORK_CONTAINER_SOCKET": "/run/engine.sock",
	})

	loaded, err := config.Load(getenv, "2.0.0", "")

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.InstanceURL != "https://other.example.com" || loaded.GithubToken != "ghp-env" || !loaded.HasGithubCredentials() || loaded.SandboxImage != "my/image:1" ||
		!loaded.CustomImage || loaded.MaxConcurrency != 5 || loaded.GitBaseURL != "file:///srv/git" || loaded.GithubAPIURL != "http://api.test" || loaded.EngineSocket != "/run/engine.sock" {
		t.Errorf("config = %+v", loaded)
	}
}

func TestLoadWithoutARegistrationFails(t *testing.T) {
	getenv := envFrom(map[string]string{"GENWORK_RUNNER_HOME": t.TempDir()})

	_, err := config.Load(getenv, "2.0.0", "")

	if !errors.Is(err, config.ErrNotRegistered) {
		t.Errorf("error = %v", err)
	}
}

func TestLoadNeedsTheInstanceURLWithNoFallback(t *testing.T) {
	home := t.TempDir()
	if err := config.Save(home, config.Stored{RunnerSecret: "secret"}); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(envFrom(map[string]string{"GENWORK_RUNNER_HOME": home}), "2.0.0", "")

	if !errors.Is(err, config.ErrNotRegistered) {
		t.Errorf("error = %v", err)
	}
}

func TestInvalidConcurrencyIsRejected(t *testing.T) {
	for _, value := range []string{"0", "65", "many"} {
		_, getenv := registered(t, map[string]string{"GENWORK_RUNNER_CONCURRENCY": value})
		if _, err := config.Load(getenv, "2.0.0", ""); err == nil || !strings.Contains(err.Error(), "GENWORK_RUNNER_CONCURRENCY") {
			t.Errorf("%q: error = %v", value, err)
		}
	}
}

func TestProfilesAreIsolatedPerAccount(t *testing.T) {
	_, getenv := registered(t, nil)
	loaded, _ := config.Load(getenv, "2.0.0", "")

	one, two := loaded.ProfileDir("account-one"), loaded.ProfileDir("account-two")
	escape := loaded.ProfileDir("../../etc")

	if one == two || filepath.Dir(one) != loaded.ProfilesDir() || filepath.Dir(escape) != loaded.ProfilesDir() {
		t.Errorf("profiles = %s %s %s", one, two, escape)
	}
	if loaded.WorkspacesDir() != filepath.Join(loaded.Home, "runs") {
		t.Errorf("workspaces = %s", loaded.WorkspacesDir())
	}
}

func TestHomeFallsBackToTheUsersHome(t *testing.T) {
	t.Setenv("HOME", "/home/someone")

	home, err := config.Home(envFrom(nil))

	if err != nil || home != "/home/someone/.genwork-runner" {
		t.Errorf("home = %q, err = %v", home, err)
	}
}

func TestLoadStoredOfAMissingFileIsEmpty(t *testing.T) {
	stored, err := config.LoadStored(t.TempDir())

	if err != nil || stored.RunnerSecret != "" {
		t.Errorf("stored = %+v, err = %v", stored, err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "runner.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadStored(home); err == nil {
		t.Error("a corrupt file was accepted")
	}
}
