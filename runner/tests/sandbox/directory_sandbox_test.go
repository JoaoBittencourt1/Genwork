package sandbox_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

func TestDirectorySandboxRunsInTheWorkspaceWithoutTheRunnersSecrets(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghp-runner-secret")
	t.Setenv("GENWORK_RUNNER_SECRET", "runner-secret")
	launcher := sandbox.NewDirectorySandbox()
	spec := sandbox.Spec{RunToken: "run-1", Workspace: t.TempDir(), Env: map[string]string{"RUN_KEY": "value"}, Command: []string{"sh", "-c", "pwd; env"}}
	var output bytes.Buffer

	result, err := launcher.Run(context.Background(), spec, &output)

	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	text := output.String()
	if !strings.Contains(text, spec.Workspace) || !strings.Contains(text, "RUN_KEY=value") {
		t.Errorf("output = %s", text)
	}
	if strings.Contains(text, "ghp-runner-secret") || strings.Contains(text, "runner-secret") {
		t.Errorf("the runner's secrets reached the process: %s", text)
	}
}

func TestDirectorySandboxReportsExitCodes(t *testing.T) {
	launcher := sandbox.NewDirectorySandbox()

	result, err := launcher.Run(context.Background(), sandbox.Spec{RunToken: "r", Workspace: t.TempDir(), Command: []string{"sh", "-c", "exit 3"}}, &bytes.Buffer{})

	if err != nil || result.ExitCode != 3 {
		t.Errorf("result = %+v, err = %v", result, err)
	}
	if _, err := launcher.Run(context.Background(), sandbox.Spec{RunToken: "r"}, &bytes.Buffer{}); err == nil {
		t.Error("an empty command was accepted")
	}
	if _, err := launcher.Run(context.Background(), sandbox.Spec{RunToken: "r", Workspace: t.TempDir(), Command: []string{"definitely-not-installed"}}, &bytes.Buffer{}); err == nil {
		t.Error("a missing program was accepted")
	}
}

func TestDirectorySandboxStopEndsTheProcess(t *testing.T) {
	launcher := sandbox.NewDirectorySandbox()
	spec := sandbox.Spec{RunToken: "run-1", Workspace: t.TempDir(), Command: []string{"sleep", "30"}}
	finished := make(chan sandbox.Result, 1)
	go func() {
		result, _ := launcher.Run(context.Background(), spec, &bytes.Buffer{})
		finished <- result
	}()
	time.Sleep(200 * time.Millisecond)

	if err := launcher.Stop(context.Background(), "run-1"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	select {
	case result := <-finished:
		if result.ExitCode == 0 {
			t.Errorf("a stopped process exited cleanly: %+v", result)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the process kept running after Stop")
	}
	if err := launcher.Stop(context.Background(), "unknown"); err != nil {
		t.Errorf("stopping an unknown run: %v", err)
	}
}

func TestDirectorySandboxDescribesItselfAsDirectoryMode(t *testing.T) {
	launcher := sandbox.NewDirectorySandbox()

	info, err := launcher.Describe(context.Background(), "")

	if err != nil || !info.DirectoryMode || info.Engine != "directory mode" {
		t.Errorf("info = %+v, err = %v", info, err)
	}
	if removed, err := launcher.RemoveLeftovers(context.Background(), nil); err != nil || removed != nil {
		t.Errorf("removed = %v, err = %v", removed, err)
	}
}

func TestFindSocketPrefersTheConfiguredSocket(t *testing.T) {
	directory, err := os.MkdirTemp("", "sock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	path := filepath.Join(directory, "engine.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	found, err := sandbox.FindSocket(path, func(string) string { return "" })

	if err != nil || found != path {
		t.Errorf("found = %q, err = %v", found, err)
	}
	fromDockerHost, err := sandbox.FindSocket("", func(name string) string {
		if name == "DOCKER_HOST" {
			return "unix://" + path
		}
		return ""
	})
	if err != nil || fromDockerHost != path {
		t.Errorf("from DOCKER_HOST = %q, err = %v", fromDockerHost, err)
	}
}

func TestFindSocketFailsWithBothOptionsWhenThereIsNoEngine(t *testing.T) {
	_, err := sandbox.FindSocket("/nonexistent.sock", func(string) string { return "" })

	if err == nil || !strings.Contains(err.Error(), "--directory-mode") {
		// A machine with a real engine socket legitimately finds one.
		if err != nil {
			t.Errorf("error = %v", err)
		}
	}
}
