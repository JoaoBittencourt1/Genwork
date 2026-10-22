package commands

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func releasePrefix(workspace Workspace) string {
	return "docker compose -f " + workspace.ComposeFile() + " "
}

func TestInstallPullsAndStartsThePublishedImagesThenEnablesTheModules(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	runner := &fakeRunner{}

	err, output := execute(t, workspace, runner, &fakePrompter{}, "install",
		"--public-url", "https://genwork.example.com/", "--instance-name", "Acme", "--modules", "gentasks")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	expectRuns(t, workspace, runner, releasePrefix(workspace), []string{
		"--profile gentasks pull",
		"up -d --wait postgres",
		"exec -T postgres psql -X -q -U genwork -d genwork",
		"--profile gentasks up -d --wait",
		"up -d core",
		"exec -T core /core module enable gentasks",
	})
	if workspace.StackEnvValue("GENWORK_PUBLIC_URL") != "https://genwork.example.com" || workspace.StackEnvValue("GENWORK_INSTANCE_NAME") != "Acme" {
		t.Errorf("public URL = %q, instance = %q", workspace.StackEnvValue("GENWORK_PUBLIC_URL"), workspace.StackEnvValue("GENWORK_INSTANCE_NAME"))
	}
	if config := installedModules(t, workspace); !slices.Equal(config.Modules, []string{"gentasks"}) {
		t.Errorf("modules = %v", config.Modules)
	}
	if !strings.Contains(output, "genwork auth") {
		t.Errorf("the output must point to creating the first administrator: %q", output)
	}
}

func TestInstallGeneratesSecretsWithoutPrintingThem(t *testing.T) {
	workspace := newReleaseWorkspace(t)

	err, output := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "install", "--public-url", "https://genwork.example.com")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	for _, name := range []string{"JWT_SIGNING_KEY", "GENWORK_MODULE_SECRET", "GENWORK_UPDATER_SECRET"} {
		value := workspace.StackEnvValue(name)
		if len(value) < 22 || strings.Contains(output, value) {
			t.Errorf("%s = %q must be generated and never printed", name, value)
		}
	}
}

func TestInstallAsksAgainUntilThePublicURLIsAbsolute(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	prompter := &fakePrompter{lines: []string{"genwork.example.com", "https://genwork.example.com", "", ""}}
	selector := &fakeSelector{checked: []string{"gentasks"}}

	err, output := executeCommand(t, workspace, &fakeRunner{}, prompter, selector, "install")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	if !strings.Contains(output, "must start with http:// or https://") {
		t.Errorf("output = %q", output)
	}
	if workspace.StackEnvValue("GENWORK_PUBLIC_URL") != "https://genwork.example.com" || workspace.StackEnvValue("GENWORK_INSTANCE_NAME") != "Genwork" {
		t.Errorf("public URL = %q, instance = %q", workspace.StackEnvValue("GENWORK_PUBLIC_URL"), workspace.StackEnvValue("GENWORK_INSTANCE_NAME"))
	}
	if !selector.checklistShown || !installedModules(t, workspace).IsInstalled("gentasks") {
		t.Error("an interactive install shows the module checklist")
	}
}

func TestInstallAsksForSMTPAndWarnsWhenMailStaysOnTheDevelopmentCatcher(t *testing.T) {
	configured := newReleaseWorkspace(t)
	prompter := &fakePrompter{
		lines:     []string{"https://genwork.example.com", "Acme", "smtp.example.com", "587", "Acme <no-reply@example.com>", "starttls", "mailer"},
		passwords: []string{"smtp-password"},
	}
	if err, output := executeCommand(t, configured, &fakeRunner{}, prompter, &fakeSelector{}, "install"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	if configured.StackEnvValue("SMTP_HOST") != "smtp.example.com" || configured.StackEnvValue("SMTP_PORT") != "587" ||
		configured.StackEnvValue("SMTP_TLS") != "starttls" || configured.StackEnvValue("SMTP_USERNAME") != "mailer" ||
		configured.StackEnvValue("SMTP_PASSWORD") != "smtp-password" || configured.StackEnvValue("SMTP_FROM") != "Acme <no-reply@example.com>" {
		t.Errorf("SMTP settings were not recorded")
	}

	skipped := newReleaseWorkspace(t)
	prompter = &fakePrompter{lines: []string{"https://genwork.example.com", "", ""}}
	err, output := executeCommand(t, skipped, &fakeRunner{}, prompter, &fakeSelector{}, "install")
	if err != nil || !strings.Contains(output, "SMTP_HOST") || skipped.StackEnvValue("SMTP_HOST") != "mailpit" {
		t.Errorf("error = %v, output = %q", err, output)
	}
}

func TestInstallWithoutATerminalNeedsThePublicURL(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	runner := &fakeRunner{}

	err, _ := execute(t, workspace, runner, &fakePrompter{}, "install")

	if err == nil || !strings.Contains(err.Error(), "--public-url") || len(runner.runs) != 0 {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
}

func TestInstallRejectsAnInvalidPublicURLFlag(t *testing.T) {
	workspace := newReleaseWorkspace(t)

	err, _ := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "install", "--public-url", "ftp://genwork.example.com")

	if err == nil || !strings.Contains(err.Error(), "http:// or https://") {
		t.Errorf("error = %v", err)
	}
}

func TestInstallRefusesAnExistingInstallation(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	installModules(t, workspace, "gentasks")
	if err := os.WriteFile(workspace.StackEnvFile(), []byte("JWT_SIGNING_KEY=kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}

	err, _ := execute(t, workspace, runner, &fakePrompter{}, "install", "--public-url", "https://genwork.example.com")

	if !errors.Is(err, ErrAlreadyInstalled) || !strings.Contains(err.Error(), "genwork update") || len(runner.runs) != 0 {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
	if content, _ := os.ReadFile(workspace.StackEnvFile()); string(content) != "JWT_SIGNING_KEY=kept\n" {
		t.Errorf("the existing .env must stay unchanged: %q", content)
	}
}

func TestInstallRefusesARepositoryCheckout(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{}

	err, _ := execute(t, workspace, runner, &fakePrompter{}, "install", "--public-url", "https://genwork.example.com")

	if !errors.Is(err, ErrInstallInRepository) || len(runner.runs) != 0 {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
}

func TestInstallFromACheckoutKeepsItsSecretsAndModules(t *testing.T) {
	checkout := newTestWorkspace(t)
	installModules(t, checkout, "gentasks")
	checkoutEnv := "JWT_SIGNING_KEY=kept-signing-key\nGENWORK_MODULE_SECRET=kept-module-secret-0123456789\n" +
		"GENWORK_PUBLIC_URL=https://staging.example.com\nPOSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nGENTASKS_DB_PASSWORD=kept-gentasks-password\n"
	if err := os.WriteFile(checkout.StackEnvFile(), []byte(checkoutEnv), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := newReleaseWorkspace(t)
	runner := &fakeRunner{}

	err, output := execute(t, workspace, runner, &fakePrompter{}, "install", "--from-checkout", checkout.Root)

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	for name, want := range map[string]string{
		"JWT_SIGNING_KEY": "kept-signing-key", "GENWORK_MODULE_SECRET": "kept-module-secret-0123456789",
		"GENWORK_PUBLIC_URL": "https://staging.example.com", "GENTASKS_DB_PASSWORD": "kept-gentasks-password",
		"GENWORK_HOME": workspace.Root,
	} {
		if got := workspace.StackEnvValue(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if workspace.StackEnvValue("GENWORK_UPDATER_SECRET") == "" {
		t.Error("keys the release needs must be added")
	}
	if !installedModules(t, workspace).IsInstalled("gentasks") || !slices.Contains(runner.commandLines(), releasePrefix(workspace)+"exec -T core /core module enable gentasks") {
		t.Errorf("the checkout's modules must be installed: %q", runner.commandLines())
	}
	if checkoutAfter, _ := os.ReadFile(checkout.StackEnvFile()); string(checkoutAfter) != checkoutEnv {
		t.Error("the checkout's files must stay unchanged")
	}
}

func TestInstallFromACheckoutWithoutConfiguration(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	empty := t.TempDir()

	err, _ := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "install", "--from-checkout", empty)

	if err == nil || !strings.Contains(err.Error(), filepath.Join(empty, "deploy", ".env")) {
		t.Errorf("error = %v", err)
	}
}

func TestInstallExplainsAFailedPull(t *testing.T) {
	workspace := newReleaseWorkspace(t)
	runner := &fakeRunner{failOn: "pull"}

	err, output := execute(t, workspace, runner, &fakePrompter{}, "install", "--public-url", "https://genwork.example.com")

	if !errors.Is(err, ErrReported) || !strings.Contains(output, "denied") || len(runner.runs) != 1 {
		t.Errorf("error = %v, output = %q, runs = %q", err, output, runner.commandLines())
	}
}
