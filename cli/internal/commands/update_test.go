package commands

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

func TestUpdateCheckReportsTheNewerReleaseWithoutChangingAnything(t *testing.T) {
	fixture := newUpdateFixture(t)

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update", "--check")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	if !strings.Contains(output, "0.5.2") || !strings.Contains(output, "https://github.com/acme/genwork/releases/tag/v0.5.2") {
		t.Errorf("output = %q", output)
	}
	if len(fixture.runner.runs) != 0 || fixture.workspace.StackEnvValue("GENWORK_MODULE_SECRET") != "kept-module-secret-0123456789" {
		t.Errorf("a check runs nothing: %q", fixture.runner.commandLines())
	}
	if content, _ := os.ReadFile(fixture.workspace.VersionFile()); string(content) != "0.5.0\n" {
		t.Errorf("VERSION = %q", content)
	}
}

func TestUpdateCheckWhenUpToDate(t *testing.T) {
	fixture := newUpdateFixture(t)
	writeTestFile(t, fixture.workspace.VersionFile(), releasestest.LatestVersion+"\n")

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update", "--check")

	if err != nil || !strings.Contains(output, "up to date") {
		t.Errorf("error = %v, output = %q", err, output)
	}
}

func TestUpdateCheckReportsAnUnreachableChannel(t *testing.T) {
	fixture := newUpdateFixture(t)
	env, _ := os.ReadFile(fixture.workspace.StackEnvFile())
	refused := "http://127.0.0.1:1"
	writeTestFile(t, fixture.workspace.StackEnvFile(), strings.Replace(string(env), "GITHUB_API_URL="+fixture.github.APIURL(), "GITHUB_API_URL="+refused, 1))

	err, _ := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update", "--check")

	if err == nil || !strings.Contains(err.Error(), "release_channel_unreachable") {
		t.Errorf("error = %v", err)
	}
}

func TestUpdateNeedsTheReleaseChannel(t *testing.T) {
	fixture := newUpdateFixture(t)
	if err := fixture.workspace.SetStackEnvValues(map[string]string{"GENWORK_RELEASE_REPOSITORY": ""}); err != nil {
		t.Fatal(err)
	}

	err, _ := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update", "--check")

	if err == nil || !strings.Contains(err.Error(), "GENWORK_RELEASE_REPOSITORY") {
		t.Errorf("error = %v", err)
	}
}

func TestUpdateInstallsTheLatestReleaseStepByStep(t *testing.T) {
	fixture := newUpdateFixture(t)

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update")

	if err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	prefix := "docker compose -f " + fixture.workspace.ComposeFile() + " "
	pulls := fixture.runsMatching(" pull")
	if len(pulls) != 1 || !strings.Contains(pulls[0].commandLine(), "--project-directory "+filepath.Join(fixture.workspace.Root, "deploy")) ||
		!strings.Contains(pulls[0].commandLine(), "--profile gentasks pull") || pulls[0].version() != releasestest.LatestVersion {
		t.Fatalf("pull = %+v", pulls)
	}
	lines := fixture.runner.commandLines()
	for _, expected := range []string{
		prefix + "exec -T postgres pg_dump -Fc -U genwork -d genwork",
		prefix + "--profile gentasks up -d --wait --wait-timeout 300 --remove-orphans postgres core-migrate core core-worker gentasks-migrate gentasks gateway",
		prefix + "up -d --no-deps --force-recreate updater",
	} {
		if !slices.Contains(lines, expected) {
			t.Errorf("missing %q in %q", expected, lines)
		}
	}
	if content, _ := os.ReadFile(fixture.workspace.VersionFile()); string(content) != "0.5.2\n" {
		t.Errorf("VERSION = %q", content)
	}
	if !strings.Contains(output, "succeeded") {
		t.Errorf("output = %q", output)
	}
}

func TestUpdateAddsTheNewReleasesKeysAndKeepsEveryValue(t *testing.T) {
	fixture := newUpdateFixture(t)

	if err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	if secret := fixture.workspace.StackEnvValue("NEW_RELEASE_SECRET"); len(secret) < 22 {
		t.Errorf("the new secret must be generated: %q", secret)
	}
	if fixture.workspace.StackEnvValue("NEW_RELEASE_SETTING") != "enabled" {
		t.Error("a new setting takes the example's value")
	}
	for name, kept := range map[string]string{
		"JWT_SIGNING_KEY": "kept-signing-key", "GENWORK_MODULE_SECRET": "kept-module-secret-0123456789",
		"GENTASKS_DB_PASSWORD": "kept-gentasks-password",
	} {
		if fixture.workspace.StackEnvValue(name) != kept {
			t.Errorf("%s = %q, want it kept", name, fixture.workspace.StackEnvValue(name))
		}
	}
	caddyfile, _ := os.ReadFile(fixture.workspace.GatewayConfigFile())
	if !strings.Contains(string(caddyfile), "handle /api/gentasks/v1/*") {
		t.Errorf("the gateway routes must be rendered again: %s", caddyfile)
	}
}

func TestUpdateReplacesItsOwnBinary(t *testing.T) {
	fixture := newUpdateFixture(t)

	if err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	if content, _ := os.ReadFile(fixture.executable); string(content) != releasestest.ExecutableContent {
		t.Errorf("binary = %q", content)
	}
}

func TestUpdateExplainsHowToReplaceAProtectedBinary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to read-only directories")
	}
	fixture := newUpdateFixture(t)
	if err := os.Chmod(filepath.Dir(fixture.executable), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(fixture.executable), 0o755) })

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update")

	if err != nil {
		t.Fatalf("the installation update must still succeed: %v (%s)", err, output)
	}
	if !strings.Contains(output, fixture.executable) || !strings.Contains(output, "still") {
		t.Errorf("output = %q", output)
	}
	if content, _ := os.ReadFile(fixture.executable); string(content) != "old binary" {
		t.Errorf("binary = %q", content)
	}
}

func TestUpdateRollsBackAFailedMigration(t *testing.T) {
	fixture := newUpdateFixture(t)
	fixture.runner.failWhen = func(run recordedRun) bool {
		return slices.Contains(run.args, "--wait") && run.version() == releasestest.LatestVersion
	}

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update")

	if !errors.Is(err, ErrReported) || !strings.Contains(output, "migrate") || !strings.Contains(output, "back on 0.5.0") {
		t.Errorf("error = %v, output = %q", err, output)
	}
	restores := fixture.runsMatching("pg_restore --clean --if-exists --single-transaction -U genwork -d genwork")
	if len(restores) != 1 || restores[0].stdin != "dump of 0.5.0" {
		t.Errorf("restores = %+v", restores)
	}
	if content, _ := os.ReadFile(fixture.workspace.VersionFile()); string(content) != "0.5.0\n" {
		t.Errorf("VERSION = %q", content)
	}
}

func TestUpdatePrintsTheRecoveryStepsWhenTheRollbackFails(t *testing.T) {
	fixture := newUpdateFixture(t)
	fixture.runner.failWhen = func(run recordedRun) bool {
		return (slices.Contains(run.args, "--wait") && run.version() == releasestest.LatestVersion) || slices.Contains(run.args, "pg_restore")
	}

	err, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "update")

	if !errors.Is(err, ErrReported) || !strings.Contains(output, filepath.Join(fixture.workspace.Root, "backups")) || !strings.Contains(output, "pg_restore") {
		t.Errorf("error = %v, output = %q", err, output)
	}
}

func TestUpdateRefusesARepositoryCheckout(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{}

	err, _ := execute(t, workspace, runner, &fakePrompter{}, "update")

	if !errors.Is(err, ErrUpdateInRepository) || len(runner.runs) != 0 {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
}

func TestAnOutdatedBinaryWarnsOnEveryCommand(t *testing.T) {
	fixture := newUpdateFixture(t)
	writeTestFile(t, fixture.workspace.VersionFile(), "0.5.2\n")
	previous := Version
	Version = "0.5.0"
	t.Cleanup(func() { Version = previous })

	_, output := execute(t, fixture.workspace, fixture.runner, &fakePrompter{}, "down")

	if !strings.Contains(output, "Warning: this genwork binary is 0.5.0") || !strings.Contains(output, "0.5.2") {
		t.Errorf("output = %q", output)
	}
	_, output = execute(t, newTestWorkspace(t), &fakeRunner{}, &fakePrompter{}, "down")
	if strings.Contains(output, "Warning: this genwork binary") {
		t.Errorf("a repository checkout builds its own binary: %q", output)
	}
}
