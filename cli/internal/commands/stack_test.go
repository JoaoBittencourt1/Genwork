package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStackCommandsRunCompose(t *testing.T) {
	cases := map[string]string{
		"db":   "up -d --build postgres",
		"api":  "up -d --build core core-worker",
		"down": "down",
	}
	for command, expectedSuffix := range cases {
		t.Run(command, func(t *testing.T) {
			workspace := newTestWorkspace(t)
			runner := &fakeRunner{}
			if err, output := execute(t, workspace, runner, &fakePrompter{}, command); err != nil {
				t.Fatalf("error: %v (%s)", err, output)
			}
			expected := "docker compose -f " + workspace.ComposeFile() + " " + expectedSuffix
			if len(runner.runs) != 1 || runner.runs[0].commandLine() != expected || runner.runs[0].dir != workspace.Root {
				t.Errorf("runs = %+v, want %q", runner.runs, expected)
			}
		})
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
	for _, expected := range []string{"http://localhost:8484", "http://localhost:8486", "localhost:5484"} {
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
	runner := &fakeRunner{}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "up"); err != nil {
		t.Fatal(err)
	}
	if len(runner.runs) != 2 {
		t.Fatalf("runs = %+v", runner.runs)
	}
	if !strings.HasSuffix(runner.runs[0].commandLine(), "up -d --build") || runner.runs[1].commandLine() != "npm run dev" {
		t.Errorf("runs = %+v", runner.runs)
	}
}

func TestUpStopsWhenTheStackFails(t *testing.T) {
	workspace := newTestWorkspace(t)
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
