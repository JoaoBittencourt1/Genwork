package commands

import (
	"errors"
	"strings"
	"testing"
)

func TestAuthAsksForEverythingAndCreatesAdministrator(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{runningServices: "postgres\ncore\n"}
	prompter := &fakePrompter{
		lines:     []string{"", "ana@example.com", "Ana Souza"},
		passwords: []string{"correct horse battery", "correct horse battery"},
	}

	if err, output := execute(t, workspace, runner, prompter, "auth"); err != nil {
		t.Fatalf("error: %v (%s)", err, output)
	}
	create := runner.runs[len(runner.runs)-1]
	expected := "docker compose -f " + workspace.ComposeFile() + " exec -T core /core admin create --email ana@example.com --name Ana Souza --password-stdin"
	if create.commandLine() != expected {
		t.Errorf("command = %q", create.commandLine())
	}
	if create.stdin != "correct horse battery\n" {
		t.Errorf("stdin = %q", create.stdin)
	}
	for _, arg := range create.args {
		if strings.Contains(arg, "correct horse battery") {
			t.Fatal("the password must never be passed as an argument")
		}
	}
}

func TestAuthUsesFlagsWithoutAsking(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{runningServices: "core\n"}
	prompter := &fakePrompter{passwords: []string{"correct horse battery", "correct horse battery"}}

	if err, _ := execute(t, workspace, runner, prompter, "auth", "--email", "ana@example.com", "--name", "Ana"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runner.runs[len(runner.runs)-1].commandLine(), "--email ana@example.com --name Ana") {
		t.Errorf("runs = %+v", runner.runs)
	}
}

func TestAuthRejectsMismatchedPasswords(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{runningServices: "core\n"}
	prompter := &fakePrompter{lines: []string{"ana@example.com", "Ana"}, passwords: []string{"correct horse battery", "correct horse batterY"}}

	if err, _ := execute(t, workspace, runner, prompter, "auth"); !errors.Is(err, ErrPasswordsDoNotMatch) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.runs) != 1 {
		t.Errorf("core must not be called after a mismatch: %+v", runner.runs)
	}
}

func TestAuthRequiresRunningAPI(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{runningServices: "postgres\n"}
	if err, _ := execute(t, workspace, runner, &fakePrompter{}, "auth"); !errors.Is(err, ErrStackNotRunning) {
		t.Fatalf("error = %v", err)
	}
}

func TestAuthReportsCoreFailureWithoutRepeatingIt(t *testing.T) {
	workspace := newTestWorkspace(t)
	runner := &fakeRunner{runningServices: "core\n", failOn: "exec"}
	prompter := &fakePrompter{lines: []string{"ana@example.com", "Ana"}, passwords: []string{"short", "short"}}
	if err, _ := execute(t, workspace, runner, prompter, "auth"); !errors.Is(err, ErrReported) {
		t.Fatalf("error = %v", err)
	}
}
