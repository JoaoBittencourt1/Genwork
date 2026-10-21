package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStackCommandsRunComposeWithoutModules(t *testing.T) {
	cases := map[string][]string{
		"db":   {"up -d --wait postgres"},
		"api":  {"up -d --wait postgres", "up -d --build core core-worker gateway", "ps --status running --services"},
		"down": {"--profile * down"},
	}
	for command, expectedSuffixes := range cases {
		t.Run(command, func(t *testing.T) {
			workspace := newTestWorkspace(t)
			installModules(t, workspace)
			runner := &fakeRunner{}
			if err, output := execute(t, workspace, runner, &fakePrompter{}, command); err != nil {
				t.Fatalf("error: %v (%s)", err, output)
			}
			expectComposeRuns(t, workspace, runner, expectedSuffixes)
		})
	}
}

func expectComposeRuns(t *testing.T, workspace Workspace, runner *fakeRunner, expectedSuffixes []string) {
	t.Helper()
	if len(runner.runs) != len(expectedSuffixes) {
		t.Fatalf("runs = %q, want suffixes %q", runner.commandLines(), expectedSuffixes)
	}
	for index, suffix := range expectedSuffixes {
		expected := "docker compose -f " + workspace.ComposeFile() + " " + suffix
		if runner.runs[index].commandLine() != expected || runner.runs[index].dir != workspace.Root {
			t.Errorf("run %d = %q, want %q", index, runner.runs[index].commandLine(), expected)
		}
	}
}

func TestAPIStartsInstalledModulesWithTheirProfiles(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	runner := &fakeRunner{runningServices: "core\ngateway\n"}

	if err, output := execute(t, workspace, runner, &fakePrompter{}, "api"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	expectComposeRuns(t, workspace, runner, []string{
		"up -d --wait postgres",
		"exec -T postgres psql -X -q -U genwork -d genwork",
		"--profile gentasks up -d --build core core-worker gateway gentasks-migrate gentasks",
		"ps --status running --services",
		"restart gateway",
	})
	caddyfile, _ := os.ReadFile(workspace.GatewayConfigFile())
	if !strings.Contains(string(caddyfile), "handle /api/gentasks/v1/* {") || strings.Contains(string(caddyfile), "reports") {
		t.Errorf("Caddyfile = %s", caddyfile)
	}
	if workspace.StackEnvValue("GENTASKS_DB_PASSWORD") == "" {
		t.Error("the installed module needs its database password")
	}
}

func TestProvisioningSendsThePasswordOnlyThroughStdin(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	runner := &fakeRunner{}
	if err, output := execute(t, workspace, runner, &fakePrompter{}, "db"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	password := workspace.StackEnvValue("GENTASKS_DB_PASSWORD")
	provisioning := runner.runs[1]

	if password == "" || strings.Contains(provisioning.commandLine(), password) {
		t.Errorf("password %q leaked into arguments: %s", password, provisioning.commandLine())
	}
	for _, expected := range []string{
		`\set module_name 'gentasks'`,
		`\set module_password '` + password + `'`,
		"CREATE SCHEMA IF NOT EXISTS :\"module_name\" AUTHORIZATION :\"module_name\"",
		"WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'module_name')",
		"REVOKE ALL ON SCHEMA public FROM :\"module_name\"",
	} {
		if !strings.Contains(provisioning.stdin, expected) {
			t.Errorf("provisioning script is missing %q:\n%s", expected, provisioning.stdin)
		}
	}
}

func TestStackStopsWhenProvisioningFails(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	runner := &fakeRunner{failOn: "exec"}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "api"); !errors.Is(err, ErrReported) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.runs) != 2 {
		t.Errorf("services must not start after a failed provisioning: %q", runner.commandLines())
	}
}

func TestStartCommandsPrepareEnvAndPrintEndpoints(t *testing.T) {
	workspace := newTestWorkspace(t)
	err, output := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "api")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(workspace.StackEnvFile()); statErr != nil {
		t.Error("deploy/.env should be created")
	}
	for _, expected := range []string{"http://localhost:8487", "http://localhost:8484", "http://localhost:8486", "localhost:5484"} {
		if !strings.Contains(output, expected) {
			t.Errorf("output %q is missing %s", output, expected)
		}
	}
}

func TestWebInstallsDependenciesOnlyWhenMissing(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "web"); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 2 || runner.runs[0].commandLine() != "npm install" || runner.runs[1].commandLine() != "npm run dev" {
		t.Fatalf("runs = %+v", runner.runs)
	}
	if runner.runs[1].dir != workspace.WebDir() {
		t.Errorf("dev server dir = %s", runner.runs[1].dir)
	}

	if err := os.Mkdir(filepath.Join(workspace.WebDir(), "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner = &fakeRunner{}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "web"); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 1 || runner.runs[0].commandLine() != "npm run dev" {
		t.Fatalf("runs = %+v", runner.runs)
	}
}

func TestUpStartsEverythingThenTheWebApp(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := os.Mkdir(filepath.Join(workspace.WebDir(), "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	installModules(t, workspace)
	runner := &fakeRunner{}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "up"); err != nil {
		t.Fatal(err)
	}
	lines := runner.commandLines()
	if len(lines) != 4 || !strings.HasSuffix(lines[1], "up -d --build") || lines[3] != "npm run dev" {
		t.Errorf("runs = %q", lines)
	}
}

func TestUpStopsWhenTheStackFails(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace)
	runner := &fakeRunner{failOn: "up"}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "up"); !errors.Is(err, ErrReported) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.runs) != 1 {
		t.Errorf("the web app must not start after a failed stack: %+v", runner.runs)
	}
}

func TestCommandsFailOutsideAWorkspace(t *testing.T) {
	root := NewRootCommand(Dependencies{
		Runner:     &fakeRunner{},
		Prompter:   &fakePrompter{},
		WorkingDir: func() (string, error) { return t.TempDir(), nil },
	})
	root.SetArgs([]string{"db"})
	if err := root.Execute(); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("error = %v", err)
	}
}
