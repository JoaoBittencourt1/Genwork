package commands

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
)

func installedModules(t *testing.T, workspace Workspace) modules.InstallationConfig {
	t.Helper()
	config, err := modules.ReadInstallationConfig(workspace.InstallationFile())
	if err != nil {
		t.Fatalf("read installation: %v", err)
	}
	return config
}

func TestModulesFailsWithoutATerminal(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{}

	err, _ := execute(t, workspace, runner, &fakePrompter{}, "modules")

	if !errors.Is(err, ErrNotInteractive) || len(runner.runs) != 0 || installedModules(t, workspace).Chosen {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
}

func TestModulesChecklistShowsInstalledModulesChecked(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks", "legacy")
	selector := &fakeSelector{checked: []string{"gentasks", "legacy"}}

	err, output := executeWithSelector(t, workspace, &fakeRunner{}, selector, "modules")

	if err != nil || !strings.Contains(output, "Nothing to change.") || selector.confirmAsked {
		t.Fatalf("error = %v, output = %q", err, output)
	}
	expected := []ModuleOption{
		{ID: "gentasks", Label: "gentasks", Checked: true},
		{ID: "legacy", Label: "legacy  (missing from catalog)", Checked: true},
		{ID: "reports", Label: "reports  requires: gentasks", Checked: false},
	}
	if !slices.Equal(selector.offered, expected) {
		t.Errorf("offered = %+v", selector.offered)
	}
}

func TestCheckingAModuleInstallsItWithItsRequirements(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace)
	selector := &fakeSelector{checked: []string{"reports"}, confirm: true}
	runner := &fakeRunner{runningServices: "core\ngateway\n"}

	if err, output := executeWithSelector(t, workspace, runner, selector, "modules"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	if !strings.Contains(selector.summary, "+ gentasks  (required by reports)") || selector.deletesData {
		t.Errorf("summary = %q", selector.summary)
	}
	if config := installedModules(t, workspace); !slices.Equal(config.Modules, []string{"gentasks", "reports"}) {
		t.Errorf("installed = %v", config.Modules)
	}
	expectComposeRuns(t, workspace, runner, []string{
		"up -d --wait postgres",
		"exec -T postgres psql -X -q -U genwork -d genwork",
		"exec -T postgres psql -X -q -U genwork -d genwork",
		"--profile gentasks --profile reports up -d --build gentasks-migrate gentasks reports-migrate reports",
		"up -d core",
		"exec -T core /core module enable gentasks",
		"up -d core",
		"exec -T core /core module enable reports",
		"ps --status running --services",
		"restart gateway",
	})
	caddyfile, _ := os.ReadFile(workspace.GatewayConfigFile())
	if !strings.Contains(string(caddyfile), "{$REPORTS_UPSTREAM}") || workspace.StackEnvValue("REPORTS_DB_PASSWORD") == "" {
		t.Errorf("Caddyfile = %s", caddyfile)
	}
}

func TestUncheckingAModuleAsksBeforeDeletingItsData(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	selector := &fakeSelector{checked: nil, confirm: false}
	runner := &fakeRunner{}

	err, output := executeWithSelector(t, workspace, runner, selector, "modules")

	if err != nil || !strings.Contains(output, "Nothing changed.") {
		t.Fatalf("error = %v, output = %q", err, output)
	}
	if !selector.deletesData || !strings.Contains(selector.summary, `- gentasks: database schema "gentasks" and everything in it`) {
		t.Errorf("summary = %q, deletesData = %v", selector.summary, selector.deletesData)
	}
	if len(runner.runs) != 0 || !installedModules(t, workspace).IsInstalled("gentasks") {
		t.Errorf("a declined uninstall must change nothing: %q", runner.commandLines())
	}
}

func TestUncheckingAModuleUninstallsItAndDeletesItsData(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	if err, output := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "db"); err != nil {
		t.Fatalf("db: %v (%s)", err, output)
	}
	selector := &fakeSelector{checked: nil, confirm: true}
	runner := &fakeRunner{runningServices: "gateway\n"}

	if err, output := executeWithSelector(t, workspace, runner, selector, "modules"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	expectComposeRuns(t, workspace, runner, []string{
		"--profile gentasks rm -s -f gentasks-migrate gentasks",
		"up -d core",
		"exec -T core /core module disable gentasks",
		"up -d --wait postgres",
		"exec -T postgres psql -X -q -U genwork -d genwork",
		"ps --status running --services",
		"restart gateway",
	})
	drop := runner.runs[4].stdin
	if !strings.Contains(drop, `\set module_name 'gentasks'`) || !strings.Contains(drop, `DROP SCHEMA IF EXISTS :"module_name" CASCADE;`) ||
		!strings.Contains(drop, `DROP ROLE IF EXISTS :"module_name";`) {
		t.Errorf("drop script = %s", drop)
	}
	if installedModules(t, workspace).IsInstalled("gentasks") || workspace.StackEnvValue("GENTASKS_DB_PASSWORD") != "" {
		t.Error("the module must leave the installation and its credentials must be cleared")
	}
	caddyfile, _ := os.ReadFile(workspace.GatewayConfigFile())
	if strings.Contains(string(caddyfile), "gentasks") {
		t.Errorf("Caddyfile still routes gentasks:\n%s", caddyfile)
	}
}

func TestFailedUninstallKeepsTheModuleChecked(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks")
	runner := &fakeRunner{failOn: "exec"}

	err, _ := executeWithSelector(t, workspace, runner, &fakeSelector{confirm: true}, "modules")

	if !errors.Is(err, ErrReported) || !installedModules(t, workspace).IsInstalled("gentasks") {
		t.Errorf("error = %v, installed = %v", err, installedModules(t, workspace).Modules)
	}
}

func TestRefusedPlanChangesNothing(t *testing.T) {
	workspace := newTestWorkspace(t)
	installModules(t, workspace, "gentasks", "reports")
	selector := &fakeSelector{checked: []string{"reports"}, confirm: true}
	runner := &fakeRunner{}

	err, _ := executeWithSelector(t, workspace, runner, selector, "modules")

	if err == nil || !strings.Contains(err.Error(), `"reports" requires it`) || selector.confirmAsked || len(runner.runs) != 0 {
		t.Errorf("error = %v, runs = %q", err, runner.commandLines())
	}
}

func TestModulesFailsOnAMalformedInstallationFile(t *testing.T) {
	workspace := newTestWorkspace(t)
	if err := os.WriteFile(workspace.InstallationFile(), []byte("modules: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	selector := &fakeSelector{}

	err, _ := executeWithSelector(t, workspace, &fakeRunner{}, selector, "modules")

	if err == nil || !strings.Contains(err.Error(), "genwork.yaml") || selector.checklistShown {
		t.Errorf("error = %v", err)
	}
}

func TestFirstRunAsksForModulesOnce(t *testing.T) {
	workspace := newTestWorkspace(t)
	selector := &fakeSelector{checked: []string{"gentasks"}}
	runner := &fakeRunner{runningServices: "core\n"}

	if err, output := executeWithSelector(t, workspace, runner, selector, "api"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}

	for _, option := range selector.offered {
		if option.Checked {
			t.Errorf("first run must start with nothing checked: %+v", option)
		}
	}
	if config := installedModules(t, workspace); !slices.Equal(config.Modules, []string{"gentasks"}) {
		t.Errorf("installed = %v", config.Modules)
	}
	lines := runner.commandLines()
	if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasSuffix(line, "exec -T core /core module enable gentasks") }) {
		t.Errorf("runs = %q", lines)
	}

	second := &fakeSelector{}
	if err, _ := executeWithSelector(t, workspace, &fakeRunner{}, second, "api"); err != nil || second.checklistShown {
		t.Errorf("the second run must not ask again (err %v)", err)
	}
}

func TestFirstRunWithoutATerminalChoosesNothing(t *testing.T) {
	workspace := newTestWorkspace(t)

	err, output := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "api")

	if err != nil || !strings.Contains(output, "Run `genwork modules`") {
		t.Fatalf("error = %v, output = %q", err, output)
	}
	if config := installedModules(t, workspace); !config.Chosen || len(config.Modules) != 0 {
		t.Errorf("config = %+v", config)
	}
}
