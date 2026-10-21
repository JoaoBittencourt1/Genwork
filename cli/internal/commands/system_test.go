package commands

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecRunnerRunsInDirectoryWithStdio(t *testing.T) {
	directory := t.TempDir()
	var stdout bytes.Buffer
	err := ExecRunner{}.Run(context.Background(), RunRequest{
		Name: "sh", Args: []string{"-c", "pwd -P; cat"}, Dir: directory,
		Stdin: strings.NewReader("from stdin"), Stdout: &stdout, Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(directory)
	if !strings.Contains(stdout.String(), resolved) || !strings.HasSuffix(stdout.String(), "from stdin") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestExecRunnerReportsFailures(t *testing.T) {
	err := ExecRunner{}.Run(context.Background(), RunRequest{Name: "sh", Args: []string{"-c", "exit 3"}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected a non-zero exit to fail")
	}
}

func TestExecRunnerInterruptsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := ExecRunner{}.Run(ctx, RunRequest{Name: "sleep", Args: []string{"30"}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected the cancelled command to fail")
	}
	if time.Since(started) > 5*time.Second {
		t.Errorf("cancellation took %s", time.Since(started))
	}
}

func newScriptedPrompter(input string, password string, passwordErr error) (*TerminalPrompter, *bytes.Buffer) {
	var output bytes.Buffer
	return &TerminalPrompter{
		input:        bufio.NewReader(strings.NewReader(input)),
		output:       &output,
		readPassword: func() ([]byte, error) { return []byte(password), passwordErr },
	}, &output
}

func TestTerminalPrompterReadsTrimmedLines(t *testing.T) {
	prompter, output := newScriptedPrompter("  ana@example.com  \nAna\n", "", nil)
	first, err := prompter.PromptLine("E-mail: ")
	if err != nil || first != "ana@example.com" {
		t.Fatalf("first = %q, %v", first, err)
	}
	second, _ := prompter.PromptLine("Name: ")
	if second != "Ana" || output.String() != "E-mail: Name: " {
		t.Errorf("second = %q, output = %q", second, output.String())
	}
	if _, err := prompter.PromptLine("Extra: "); err == nil {
		t.Error("expected end of input to fail")
	}
}

func TestTerminalPrompterAcceptsLastLineWithoutNewline(t *testing.T) {
	prompter, _ := newScriptedPrompter("Ana", "", nil)
	if answer, err := prompter.PromptLine("Name: "); err != nil || answer != "Ana" {
		t.Fatalf("answer = %q, %v", answer, err)
	}
}

func TestTerminalPrompterReadsPasswordsWithoutEcho(t *testing.T) {
	prompter, output := newScriptedPrompter("", "correct horse battery", nil)
	password, err := prompter.PromptPassword("Password: ")
	if err != nil || password != "correct horse battery" {
		t.Fatalf("password = %q, %v", password, err)
	}
	if strings.Contains(output.String(), "correct horse battery") {
		t.Error("the password must not be written to the terminal")
	}

	failing, _ := newScriptedPrompter("", "", errors.New("not a terminal"))
	if _, err := failing.PromptPassword("Password: "); err == nil || !strings.Contains(err.Error(), "read password") {
		t.Errorf("error = %v", err)
	}
}

func TestDefaultDependenciesAreWired(t *testing.T) {
	dependencies := DefaultDependencies()
	if dependencies.Runner == nil || dependencies.Prompter == nil || dependencies.WorkingDir == nil {
		t.Fatalf("dependencies = %+v", dependencies)
	}
}
