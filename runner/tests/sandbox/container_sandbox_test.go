package sandbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/enginetest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

func specFor(t *testing.T, withProfile bool) sandbox.Spec {
	t.Helper()
	spec := sandbox.Spec{
		RunToken: "run-1", Image: "registry.example/genwork-sandbox:2.0.0", Workspace: t.TempDir(),
		Env:     map[string]string{"ANTHROPIC_API_KEY": "sk-run-key", "GENWORK_MCP_URL": "http://genwork.test/mcp"},
		Command: []string{"claude", "-p", "do it"}, Limits: sandbox.Limits{CPUs: 2, Memory: 4 << 30, PIDs: 512},
	}
	if withProfile {
		spec.Profile = &sandbox.Profile{Dir: t.TempDir(), Mount: "/home/sandbox/.claude"}
	}
	return spec
}

func hostConfigOf(t *testing.T, request map[string]any) map[string]any {
	t.Helper()
	config, ok := request["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("no HostConfig in %v", request)
	}
	return config
}

func TestContainerSpecIsolatesTheRun(t *testing.T) {
	spec := specFor(t, true)

	request := sandbox.BuildContainerRequest(spec)

	encoded, _ := json.Marshal(request)
	var decoded map[string]any
	_ = json.Unmarshal(encoded, &decoded)
	host := hostConfigOf(t, decoded)
	if decoded["User"] == "" || decoded["User"] == "0" || strings.HasPrefix(decoded["User"].(string), "0:") {
		t.Errorf("the sandbox runs as root: %v", decoded["User"])
	}
	if caps := host["CapDrop"].([]any); len(caps) != 1 || caps[0] != "ALL" {
		t.Errorf("CapDrop = %v", caps)
	}
	if opts := host["SecurityOpt"].([]any); len(opts) != 1 || opts[0] != "no-new-privileges" {
		t.Errorf("SecurityOpt = %v", opts)
	}
	if host["ReadonlyRootfs"] != true || host["Privileged"] == true || host["NetworkMode"] != "bridge" {
		t.Errorf("host config = %v", host)
	}
	tmpfs := host["Tmpfs"].(map[string]any)
	if _, found := tmpfs["/tmp"]; !found || tmpfs[sandbox.HomeMount] == nil {
		t.Errorf("tmpfs = %v", tmpfs)
	}
	if host["Memory"].(float64) != float64(4<<30) || host["PidsLimit"].(float64) != 512 || host["NanoCpus"].(float64) != 2e9 {
		t.Errorf("limits = %v", host)
	}
	if decoded["Labels"].(map[string]any)[sandbox.RunLabel] != "run-1" {
		t.Errorf("labels = %v", decoded["Labels"])
	}
}

func TestContainerMountsOnlyTheWorkspaceAndTheRunsOwnProfile(t *testing.T) {
	spec := specFor(t, true)

	binds := sandbox.BuildContainerRequest(spec).HostConfig.Binds

	if len(binds) != 2 || binds[0] != spec.Workspace+":/workspace:rw" || binds[1] != spec.Profile.Dir+":/home/sandbox/.claude:rw" {
		t.Errorf("binds = %v", binds)
	}
	if without := sandbox.BuildContainerRequest(specFor(t, false)).HostConfig.Binds; len(without) != 1 {
		t.Errorf("an API-key run mounts %v", without)
	}
}

func TestContainerNeverReceivesTheEngineSocketOrRunnerCredentials(t *testing.T) {
	spec := specFor(t, false)

	request := sandbox.BuildContainerRequest(spec)

	encoded, _ := json.Marshal(request)
	for _, forbidden := range []string{"docker.sock", "podman.sock", "RUNNER_SECRET", "GH_TOKEN", "GITHUB_TOKEN", "runnerSecret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("the container definition mentions %s: %s", forbidden, encoded)
		}
	}
	env := strings.Join(request.Env, " ")
	if !strings.Contains(env, "ANTHROPIC_API_KEY=sk-run-key") || !strings.Contains(env, "HOME=/home/sandbox") {
		t.Errorf("env = %v", request.Env)
	}
}

func TestRunStreamsOutputAndRemovesTheContainer(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.Logs = `{"type":"assistant","text":"hello"}` + "\n"
	launcher := sandbox.NewContainerSandbox(engine.Socket())
	var output bytes.Buffer

	result, err := launcher.Run(context.Background(), specFor(t, true), &output)

	if err != nil || result.ExitCode != 0 || result.OOMKilled {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if output.String() != engine.Logs {
		t.Errorf("output = %q", output.String())
	}
	containers := engine.Containers()
	if len(containers) != 1 || !containers[0].Started || !containers[0].Removed {
		t.Errorf("containers = %+v", containers)
	}
}

func TestOutOfMemoryMapsToTheLimitResult(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.ExitCode, engine.OOMKilled = 137, true
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	result, err := launcher.Run(context.Background(), specFor(t, false), &bytes.Buffer{})

	if err != nil || !result.OOMKilled || result.ExitCode != 137 {
		t.Errorf("result = %+v, err = %v", result, err)
	}
	if containers := engine.Containers(); !containers[0].Removed {
		t.Error("the container was not removed after the limit kill")
	}
}

func TestMissingImageIsPulled(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.HasImage = false
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	if _, err := launcher.Run(context.Background(), specFor(t, false), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(engine.Pulled) != 1 || engine.Pulled[0] != "registry.example/genwork-sandbox:2.0.0" {
		t.Errorf("pulled = %v", engine.Pulled)
	}
}

func TestLeftoverSandboxesAreRemovedExceptTheOnesInUse(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.Leftovers = map[string]string{"old-run": "leftover-1", "live-run": "leftover-2"}
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	removed, err := launcher.RemoveLeftovers(context.Background(), map[string]bool{"live-run": true})

	if err != nil || len(removed) != 1 || removed[0] != "old-run" {
		t.Errorf("removed = %v, err = %v", removed, err)
	}
	if len(engine.Leftovers) != 1 {
		t.Errorf("leftovers = %v", engine.Leftovers)
	}
}

func TestDescribeReadsToolsFromTheImageLabels(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	info, err := launcher.Describe(context.Background(), "registry.example/genwork-sandbox:2.0.0")

	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if info.Tools["claude"] != "2.1.0" || info.Tools["codex"] != "0.30.0" || len(info.Tools) != 2 || !strings.Contains(info.ImageDigest, "sha256:abc123") ||
		!strings.Contains(info.Engine, "27.0.1") || info.DirectoryMode {
		t.Errorf("info = %+v", info)
	}
}

func TestStopIsANoOpForUnknownRuns(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	if err := launcher.Stop(context.Background(), "nothing"); err != nil {
		t.Errorf("stop: %v", err)
	}
}

func TestNoEngineIsReportedWithBothOptions(t *testing.T) {
	launcher := sandbox.NewContainerSandbox("/nonexistent/engine.sock")

	_, err := launcher.Run(context.Background(), specFor(t, false), &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "directory-mode") {
		t.Errorf("error = %v", err)
	}
}

func TestSandboxLimitsAreTrimmedToASmallEngine(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.EngineCPUs, engine.EngineMemory = 1, 2<<30
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	if _, err := launcher.Run(context.Background(), specFor(t, false), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	session, err := launcher.StartInteractive(context.Background(), signInSpec(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer session.Close()

	for _, container := range engine.Containers() {
		config := hostConfigOf(t, container.Request)
		if labels, _ := container.Request["Labels"].(map[string]any); labels["genwork.run"] != nil && config["Memory"] != float64(2<<30) {
			t.Errorf("the run's memory was not trimmed: %v", config["Memory"])
		}
		if config["NanoCpus"] != float64(1e9) || config["Memory"].(float64) > float64(2<<30) {
			t.Errorf("limits were not trimmed: cpus = %v, memory = %v", config["NanoCpus"], config["Memory"])
		}
	}
}

func TestSandboxLimitsStayWhenTheEngineDoesNotSayItsCapacity(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	if _, err := launcher.Run(context.Background(), specFor(t, false), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if config := hostConfigOf(t, engine.Containers()[0].Request); config["NanoCpus"] != float64(2e9) {
		t.Errorf("cpus = %v", config["NanoCpus"])
	}
}
