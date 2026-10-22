package commands

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type recordedRun struct {
	name  string
	args  []string
	dir   string
	env   []string
	stdin string
}

func (run recordedRun) commandLine() string {
	return run.name + " " + strings.Join(run.args, " ")
}

type fakeRunner struct {
	runningServices string
	// composeServices is what `compose config --services` prints.
	composeServices string
	// stdout is printed by every command whose line contains the key, e.g. pg_dump's output.
	stdout map[string]string
	failOn string
	// failWhen fails any run it matches, e.g. `up` with one GENWORK_VERSION.
	failWhen func(run recordedRun) bool
	// onRun sees every run and whether it fails, e.g. to play the version the gateway reports after an `up`.
	onRun func(run recordedRun, failed bool)
	mutex sync.Mutex
	runs  []recordedRun
}

func (runner *fakeRunner) Run(_ context.Context, request RunRequest) error {
	run := recordedRun{name: request.Name, args: request.Args, dir: request.Dir, env: request.Env}
	if request.Stdin != nil && request.Name == "docker" {
		content, _ := io.ReadAll(request.Stdin)
		run.stdin = string(content)
	}
	runner.mutex.Lock()
	runner.runs = append(runner.runs, run)
	runner.mutex.Unlock()
	if slices.Contains(request.Args, "ps") {
		_, _ = io.WriteString(request.Stdout, runner.runningServices)
	}
	if slices.Contains(request.Args, "config") {
		_, _ = io.WriteString(request.Stdout, runner.composeServices)
	}
	for key, output := range runner.stdout {
		if strings.Contains(run.commandLine(), key) {
			_, _ = io.WriteString(request.Stdout, output)
		}
	}
	failed := (runner.failOn != "" && slices.Contains(request.Args, runner.failOn)) || (runner.failWhen != nil && runner.failWhen(run))
	if runner.onRun != nil {
		runner.onRun(run, failed)
	}
	if failed {
		return errors.New("exit status 1")
	}
	return nil
}

func (run recordedRun) version() string {
	for _, variable := range run.env {
		if version, found := strings.CutPrefix(variable, "GENWORK_VERSION="); found {
			return version
		}
	}
	return ""
}

type fakeSelector struct {
	checked        []string
	confirm        bool
	offered        []ModuleOption
	summary        string
	deletesData    bool
	confirmAsked   bool
	checklistShown bool
}

func (selector *fakeSelector) ChooseModules(_ string, options []ModuleOption) ([]string, error) {
	selector.checklistShown = true
	selector.offered = options
	return selector.checked, nil
}

func (selector *fakeSelector) ConfirmPlan(summary string, deletesData bool) (bool, error) {
	selector.confirmAsked = true
	selector.summary = summary
	selector.deletesData = deletesData
	return selector.confirm, nil
}

type fakePrompter struct {
	lines     []string
	passwords []string
}

func (prompter *fakePrompter) PromptLine(string) (string, error) {
	if len(prompter.lines) == 0 {
		return "", io.EOF
	}
	line := prompter.lines[0]
	prompter.lines = prompter.lines[1:]
	return line, nil
}

func (prompter *fakePrompter) PromptPassword(string) (string, error) {
	password := prompter.passwords[0]
	prompter.passwords = prompter.passwords[1:]
	return password, nil
}

// newTestWorkspace is a clone of the repository: it has the web sources and the dev compose override.
func newTestWorkspace(t *testing.T) Workspace {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"deploy/docker-compose.yml": "services: {}\n",
		"deploy/.env.example": "CORE_PORT=8484\nGATEWAY_PORT=8487\nMAILPIT_UI_PORT=8486\nPOSTGRES_PORT=5484\n" +
			"POSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nJWT_SIGNING_KEY=\nGENWORK_MODULE_SECRET=\nGENTASKS_DB_PASSWORD=\nGENTASKS_DATABASE_URL=\n",
		"web/.env.example":              "DEV_API_PROXY_TARGET=http://localhost:8487\nDEV_SERVER_PORT=8485\n",
		"web/package.json":              "{}\n",
		"deploy/docker-compose.dev.yml": "services: {}\n",
		"VERSION":                       "0.5.0\n",
		"modules/gentasks/module.yaml":  "id: gentasks\ncore: \">=1.0.0\"\napiPrefix: /api/gentasks/v1\n",
		"modules/reports/module.yaml": "id: reports\ncore: \">=1.0.0\"\napiPrefix: /api/reports/v1\n" +
			"requires:\n  - id: gentasks\n    version: \">=0.1.0\"\n",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Workspace{Root: root}
}

// newReleaseWorkspace is an extracted deploy bundle: VERSION, the base compose file, the env example, and the manifests.
func newReleaseWorkspace(t *testing.T) Workspace {
	t.Helper()
	workspace := newTestWorkspace(t)
	for _, path := range []string{"web", filepath.Join("deploy", "docker-compose.dev.yml")} {
		if err := os.RemoveAll(filepath.Join(workspace.Root, path)); err != nil {
			t.Fatal(err)
		}
	}
	example := "GATEWAY_PORT=8487\nCORE_PORT=8484\nPOSTGRES_PORT=5484\nPOSTGRES_USER=genwork\nPOSTGRES_DB=genwork\n" +
		"GENWORK_PUBLIC_URL=http://localhost:8485\nGENWORK_INSTANCE_NAME=Genwork\nJWT_SIGNING_KEY=\nGENWORK_MODULE_SECRET=\n" +
		"GENWORK_UPDATER_SECRET=\nGENWORK_HOME=\nSMTP_HOST=mailpit\nSMTP_PORT=1025\nSMTP_FROM=Genwork <no-reply@genwork.local>\n" +
		"SMTP_TLS=none\nSMTP_USERNAME=\nSMTP_PASSWORD=\nGENTASKS_DB_PASSWORD=\nGENTASKS_DATABASE_URL=\n"
	if err := os.WriteFile(workspace.StackEnvFile()+".example", []byte(example), 0o644); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func execute(t *testing.T, workspace Workspace, runner *fakeRunner, prompter *fakePrompter, args ...string) (error, string) {
	t.Helper()
	return executeCommand(t, workspace, runner, prompter, nil, args...)
}

// executeWithSelector runs interactively, as if a terminal were attached.
func executeWithSelector(t *testing.T, workspace Workspace, runner *fakeRunner, selector *fakeSelector, args ...string) (error, string) {
	t.Helper()
	return executeCommand(t, workspace, runner, &fakePrompter{}, selector, args...)
}

func executeCommand(t *testing.T, workspace Workspace, runner *fakeRunner, prompter *fakePrompter, selector *fakeSelector, args ...string) (error, string) {
	t.Helper()
	return executeWithDependencies(t, workspace, runner, prompter, selector, nil, args...)
}

// executeWithEnv runs with the given process environment, e.g. GENWORK_HOME.
func executeWithEnv(t *testing.T, workspace Workspace, runner *fakeRunner, environment map[string]string, args ...string) (error, string) {
	t.Helper()
	return executeWithDependencies(t, workspace, runner, &fakePrompter{}, nil, environment, args...)
}

func executeWithDependencies(t *testing.T, workspace Workspace, runner *fakeRunner, prompter *fakePrompter, selector *fakeSelector,
	environment map[string]string, args ...string,
) (error, string) {
	t.Helper()
	var output bytes.Buffer
	dependencies := Dependencies{
		Runner:        runner,
		Prompter:      prompter,
		IsInteractive: func() bool { return selector != nil },
		WorkingDir:    func() (string, error) { return filepath.Join(workspace.Root, "deploy"), nil },
		Getenv:        func(name string) string { return environment[name] },
		Executable:    func() (string, error) { return filepath.Join(workspace.Root, "bin", "genwork"), nil },
	}
	if selector != nil {
		dependencies.Selector = selector
	}
	root := NewRootCommand(dependencies)
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background()), output.String()
}

func installModules(t *testing.T, workspace Workspace, ids ...string) {
	t.Helper()
	body := "modules:\n"
	for _, id := range ids {
		body += "  - " + id + "\n"
	}
	if err := os.WriteFile(workspace.InstallationFile(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (runner *fakeRunner) commandLines() []string {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	lines := make([]string, 0, len(runner.runs))
	for _, run := range runner.runs {
		lines = append(lines, run.commandLine())
	}
	return lines
}

// composePrefix is how every compose call in a repository checkout starts: the base file plus the dev override.
func composePrefix(workspace Workspace) string {
	return "docker compose -f " + workspace.ComposeFile() + " -f " + workspace.DevComposeFile() + " "
}
