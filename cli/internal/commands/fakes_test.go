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
		"deploy/.env.example":       "CORE_PORT=8484\nMAILPIT_UI_PORT=8486\nPOSTGRES_PORT=5484\nJWT_SIGNING_KEY=\n",
		"web/.env.example":          "DEV_SERVER_PORT=8485\n",
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
	var output bytes.Buffer
	root := NewRootCommand(Dependencies{
		Runner:     runner,
		Prompter:   prompter,
		WorkingDir: func() (string, error) { return filepath.Join(workspace.Root, "web"), nil },
	})
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background()), output.String()
}
