package packaging

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWriteDeployBundleCarriesOnlyWhatAServerNeeds(t *testing.T) {
	root := newRepositoryFixture(t)
	bundle := filepath.Join(t.TempDir(), "genwork-bundle_0.5.0.tar.gz")

	if err := WriteDeployBundle(root, bundle); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	entries := readArchive(t, bundle)
	names := slices.Sorted(maps.Keys(entries))
	expected := []string{"VERSION", "deploy/.env.example", "deploy/docker-compose.yml", "modules/genmonitor/module.yaml", "modules/gentasks/module.yaml"}
	if !slices.Equal(names, expected) {
		t.Errorf("bundle = %v, want %v", names, expected)
	}
	if entries["VERSION"] != "0.5.0\n" || entries["modules/gentasks/module.yaml"] != "id: gentasks\n" {
		t.Errorf("entries = %v", entries)
	}
}

func TestWriteDeployBundleFailsWhenARequiredFileIsMissing(t *testing.T) {
	root := newRepositoryFixture(t)
	if err := os.Remove(filepath.Join(root, "deploy", ".env.example")); err != nil {
		t.Fatal(err)
	}

	err := WriteDeployBundle(root, filepath.Join(t.TempDir(), "bundle.tar.gz"))

	if err == nil || !strings.Contains(err.Error(), ".env.example") {
		t.Errorf("error = %v", err)
	}
}

func TestRenderInstallScriptTakesTheReleaseRepositoryFromTheEnvironment(t *testing.T) {
	getenv := envFrom(map[string]string{"GITHUB_SERVER_URL": "https://github.example", "GITHUB_REPOSITORY": "acme/genwork"})

	rendered, err := RenderInstallScript([]byte(installScriptTemplateFixture), "0.5.0", getenv)

	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(rendered), `RELEASE_URL="https://github.example/acme/genwork/releases"`) ||
		!strings.Contains(string(rendered), `VERSION="0.5.0"`) {
		t.Errorf("rendered = %s", rendered)
	}
}

func TestRenderInstallScriptFailsWithoutTheReleaseRepository(t *testing.T) {
	for _, missing := range []string{"GITHUB_SERVER_URL", "GITHUB_REPOSITORY"} {
		values := map[string]string{"GITHUB_SERVER_URL": "https://github.example", "GITHUB_REPOSITORY": "acme/genwork"}
		delete(values, missing)

		if _, err := RenderInstallScript([]byte(installScriptTemplateFixture), "0.5.0", envFrom(values)); err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("without %s: error = %v", missing, err)
		}
	}
}

func TestRunWritesTheBundleAndTheInstallScriptOfTheRepositoryVersion(t *testing.T) {
	root := newRepositoryFixture(t)
	output := t.TempDir()
	getenv := envFrom(map[string]string{"GITHUB_SERVER_URL": "https://github.example", "GITHUB_REPOSITORY": "acme/genwork"})

	if err := Run([]string{"bundle", root, output}, getenv, &bytes.Buffer{}); err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if err := Run([]string{"install-script", root, output}, getenv, &bytes.Buffer{}); err != nil {
		t.Fatalf("install-script: %v", err)
	}

	if entries := readArchive(t, filepath.Join(output, "genwork-bundle_0.5.0.tar.gz")); entries["VERSION"] != "0.5.0\n" {
		t.Errorf("bundle entries = %v", entries)
	}
	script, err := os.ReadFile(filepath.Join(output, "install.sh"))
	if err != nil || !strings.Contains(string(script), `VERSION="0.5.0"`) {
		t.Errorf("install.sh = %s, err = %v", script, err)
	}
	if info, _ := os.Stat(filepath.Join(output, "install.sh")); info.Mode().Perm()&0o111 == 0 {
		t.Errorf("install.sh mode = %v, want executable", info.Mode())
	}
}

func TestRunRejectsUnknownOrIncompleteCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"bundle"}, {"bundle", "root"}, {"publish", "root", "out"}} {
		if err := Run(args, envFrom(nil), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%v: error = %v", args, err)
		}
	}
}

func TestIsBundlePathAcceptsOnlyTheBundleLayout(t *testing.T) {
	for _, path := range []string{"VERSION", "deploy/docker-compose.yml", "deploy/.env.example", "modules/gentasks/module.yaml"} {
		if !IsBundlePath(path) {
			t.Errorf("%s belongs in the bundle", path)
		}
	}
	for _, path := range []string{"deploy/.env", "deploy/docker-compose.dev.yml", "../VERSION", "/VERSION", "modules/gentasks/go.mod", "modules/../module.yaml", "core/main.go"} {
		if IsBundlePath(path) {
			t.Errorf("%s must not be in the bundle", path)
		}
	}
}
