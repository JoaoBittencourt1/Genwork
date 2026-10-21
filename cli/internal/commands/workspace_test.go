package commands

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestEnsureModuleSecretsGeneratesCredentials(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureModuleSecrets(&bytes.Buffer{}, []string{"gentasks"}); err != nil {
		t.Fatal(err)
	}
	secret, password := workspace.StackEnvValue("GENWORK_MODULE_SECRET"), workspace.StackEnvValue("GENTASKS_DB_PASSWORD")
	if len(secret) < 22 || len(password) < 22 || secret == password || strings.ContainsAny(secret+password, "+/=") {
		t.Fatalf("secret = %q, password = %q", secret, password)
	}
	if workspace.StackEnvValue("GENTASKS_DATABASE_URL") != "postgres://gentasks:"+password+"@postgres:5432/genwork?sslmode=disable" {
		t.Errorf("database URL = %q", workspace.StackEnvValue("GENTASKS_DATABASE_URL"))
	}
}

func TestEnsureEnvFilesAddsMissingSecretsToAnExistingFile(t *testing.T) {
	workspace := newTestWorkspace(t)
	existing := "JWT_SIGNING_KEY=mine\nPOSTGRES_DB=genwork\nGENWORK_MODULE_SECRET=kept-secret-0123456789abcdef\n"
	if err := os.WriteFile(workspace.StackEnvFile(), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer

	if err := workspace.EnsureEnvFiles(&output); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureModuleSecrets(&output, []string{"gentasks"}); err != nil {
		t.Fatal(err)
	}

	if workspace.StackEnvValue("JWT_SIGNING_KEY") != "mine" || workspace.StackEnvValue("GENWORK_MODULE_SECRET") != "kept-secret-0123456789abcdef" {
		t.Error("existing values must be kept")
	}
	if len(workspace.StackEnvValue("GENTASKS_DB_PASSWORD")) < 22 || workspace.StackEnvValue("GENTASKS_DATABASE_URL") == "" {
		t.Error("missing values must be generated")
	}
	if !strings.Contains(output.String(), "GENTASKS_DB_PASSWORD") || strings.Contains(output.String(), workspace.StackEnvValue("GENTASKS_DB_PASSWORD")) {
		t.Errorf("output must name the variable without printing it: %q", output.String())
	}

	before, _ := os.ReadFile(workspace.StackEnvFile())
	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureModuleSecrets(&bytes.Buffer{}, []string{"gentasks"}); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(workspace.StackEnvFile()); string(after) != string(before) {
		t.Error("a second run must not change the file")
	}
}

func TestEnsureEnvFilesAppendsKeysAddedToTheExample(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := os.WriteFile(workspace.StackEnvFile(), []byte("JWT_SIGNING_KEY=mine\nPOSTGRES_DB=genwork\nCORE_PORT=9999"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	if workspace.StackEnvValue("GATEWAY_PORT") != "8487" || workspace.StackEnvValue("CORE_PORT") != "9999" {
		t.Errorf("gateway = %q, core = %q", workspace.StackEnvValue("GATEWAY_PORT"), workspace.StackEnvValue("CORE_PORT"))
	}
}

func TestClearModuleSecrets(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := workspace.EnsureEnvFiles(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureModuleSecrets(&bytes.Buffer{}, []string{"gentasks"}); err != nil {
		t.Fatal(err)
	}

	if err := workspace.ClearModuleSecrets("gentasks"); err != nil {
		t.Fatal(err)
	}

	if workspace.StackEnvValue("GENTASKS_DB_PASSWORD") != "" || workspace.StackEnvValue("GENTASKS_DATABASE_URL") != "" || workspace.StackEnvValue("GENWORK_MODULE_SECRET") == "" {
		t.Error("only the module's credentials must be cleared")
	}
}
