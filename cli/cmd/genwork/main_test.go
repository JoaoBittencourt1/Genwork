package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/commands"
)

func outsideAnyWorkspace(t *testing.T) commands.Dependencies {
	dependencies := commands.DefaultDependencies()
	directory := t.TempDir()
	dependencies.WorkingDir = func() (string, error) { return directory, nil }
	dependencies.Getenv = func(string) string { return "" }
	return dependencies
}

func TestExecuteListsCommandsInHelp(t *testing.T) {
	var stdout bytes.Buffer
	if code := execute(context.Background(), []string{"--help"}, outsideAnyWorkspace(t), &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	for _, command := range []string{"up", "db", "api", "web", "auth", "down", "version"} {
		if !strings.Contains(stdout.String(), command) {
			t.Errorf("help is missing %q", command)
		}
	}
}

func TestExecutePrintsErrorsAndFails(t *testing.T) {
	var stderr bytes.Buffer
	if code := execute(context.Background(), []string{"down"}, outsideAnyWorkspace(t), &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "error: not inside a Genwork installation or repository") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
