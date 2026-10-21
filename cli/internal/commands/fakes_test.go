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
	"testing"
)

type recordedRun struct {
	name  string
	args  []string
	dir   string
	stdin string
}

func (run recordedRun) commandLine() string {
	return run.name + " " + strings.Join(run.args, " ")
}

type fakeRunner struct {
	runningServices string
	failOn          string
	runs            []recordedRun
}

func (runner *fakeRunner) Run(_ context.Context, request RunRequest) error {
	run := recordedRun{name: request.Name, args: request.Args, dir: request.Dir}
	if request.Stdin != nil && request.Name == "docker" {
		content, _ := io.ReadAll(request.Stdin)
		run.stdin = string(content)
	}
	runner.runs = append(runner.runs, run)
	if slices.Contains(request.Args, "ps") {
		_, _ = io.WriteString(request.Stdout, runner.runningServices)
	}
	if runner.failOn != "" && slices.Contains(request.Args, runner.failOn) {
		return errors.New("exit status 1")
	}
	return nil
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

func newTestWorkspace(t *testing.T) Workspace {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"deploy/docker-compose.yml": "services: {}\n",
		"deploy/.env.example": "CORE_PORT=8484\nGATEWAY_PORT=8487\nMAILPIT_UI_PORT=8486\nPOSTGRES_PORT=5484\n" +
			"POSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nJWT_SIGNING_KEY=\nGENWORK_MODULE_SECRET=\nGENTASKS_DB_PASSWORD=\nGENTASKS_DATABASE_URL=\n",
		"web/.env.example":             "DEV_API_PROXY_TARGET=http://localhost:8487\nDEV_SERVER_PORT=8485\n",
		"modules/gentasks/module.yaml": "id: gentasks\nversion: 0.1.0\ncore: \">=1.0.0\"\napiPrefix: /api/gentasks/v1\n",
		"modules/reports/module.yaml": "id: reports\nversion: 1.0.0\ncore: \">=1.0.0\"\napiPrefix: /api/reports/v1\n" +
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
	var output bytes.Buffer
	dependencies := Dependencies{
		Runner:        runner,
		Prompter:      prompter,
		IsInteractive: func() bool { return selector != nil },
		WorkingDir:    func() (string, error) { return filepath.Join(workspace.Root, "web"), nil },
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
	lines := make([]string, 0, len(runner.runs))
	for _, run := range runner.runs {
		lines = append(lines, run.commandLine())
	}
	return lines
}
