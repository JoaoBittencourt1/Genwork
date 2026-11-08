package sandbox_test

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/enginetest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

func signInSpec(t *testing.T) sandbox.Spec {
	t.Helper()
	return sandbox.Spec{
		SignInToken: "signin-1", Image: "registry.example/genwork-sandbox:2.2.0", Workspace: t.TempDir(),
		Profile: &sandbox.Profile{Dir: t.TempDir(), Mount: "/home/sandbox/.claude"}, Env: map[string]string{"CLAUDE_CONFIG_DIR": "/home/sandbox/.claude"},
		Command: []string{"claude", "auth", "login"}, Limits: sandbox.Limits{CPUs: 1, Memory: 1 << 30, PIDs: 256},
	}
}

func TestInteractiveContainerAttachesATerminalAndTypesIntoIt(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.TerminalOutput = "Open https://claude.ai/oauth/authorize?code=true\r\nPaste code here if prompted > "
	engine.TerminalReply = "Login successful.\r\n"
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	session, err := launcher.StartInteractive(context.Background(), signInSpec(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer session.Close()
	reader := bufio.NewReader(session.Output())
	line, _ := reader.ReadString('\n')
	if !strings.Contains(line, "https://claude.ai/oauth/authorize") {
		t.Fatalf("first line = %q", line)
	}
	if _, err := io.WriteString(session.Input(), "pasted-code\r"); err != nil {
		t.Fatalf("type: %v", err)
	}
	result, err := session.Wait()

	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if engine.Typed() != "pasted-code\r" {
		t.Errorf("typed = %q", engine.Typed())
	}
	container := engine.Containers()[0]
	request := container.Request
	if request["Tty"] != true || request["OpenStdin"] != true || request["StdinOnce"] != true || request["AttachStdin"] != true {
		t.Errorf("the container has no interactive terminal: %v", request)
	}
	labels, _ := request["Labels"].(map[string]any)
	if labels["genwork.signin"] != "signin-1" || labels["genwork.run"] != nil {
		t.Errorf("labels = %v", labels)
	}
	if config := hostConfigOf(t, request); config["ReadonlyRootfs"] != true || len(config["Binds"].([]any)) != 2 {
		t.Errorf("the sign-in container is not isolated like a run: %v", config)
	}
	if len(engine.Resizes) != 1 || engine.Resizes[0] != "500x50" {
		t.Errorf("resizes = %v", engine.Resizes)
	}
}

func TestClosingAnInteractiveContainerRemovesIt(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.TerminalOutput = "waiting\r\n"
	launcher := sandbox.NewContainerSandbox(engine.Socket())
	session, err := launcher.StartInteractive(context.Background(), signInSpec(t))
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if !engine.Containers()[0].Removed {
		t.Error("the sign-in container was left behind")
	}
}

func TestLeftoverSignInContainersAreRemovedWithoutReportingRuns(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.SignInLeftovers = map[string]string{"signin-old": "leftover-signin"}
	engine.Leftovers = map[string]string{"old-run": "leftover-run"}
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	removed, err := launcher.RemoveLeftovers(context.Background(), nil)

	if err != nil || len(removed) != 1 || removed[0] != "old-run" {
		t.Errorf("removed = %v, err = %v", removed, err)
	}
	if len(engine.SignInLeftovers) != 0 || len(engine.Leftovers) != 0 {
		t.Errorf("leftovers = %v / %v", engine.SignInLeftovers, engine.Leftovers)
	}
}

func TestDirectoryInteractiveSessionReadsWhatIsTyped(t *testing.T) {
	launcher := sandbox.NewDirectorySandbox()
	spec := sandbox.Spec{SignInToken: "signin-2", Workspace: t.TempDir(), Command: []string{"sh", "-c", `echo "visit https://example.com/login"; read code; echo "got $code"`}}

	session, err := launcher.StartInteractive(context.Background(), spec)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer session.Close()
	reader := bufio.NewReader(session.Output())
	if line, _ := reader.ReadString('\n'); !strings.Contains(line, "https://example.com/login") {
		t.Fatalf("first line = %q", line)
	}
	_, _ = io.WriteString(session.Input(), "abc\n")
	reply, _ := reader.ReadString('\n')
	result, err := session.Wait()

	if err != nil || result.ExitCode != 0 || !strings.Contains(reply, "got abc") {
		t.Errorf("result = %+v, err = %v, reply = %q", result, err, reply)
	}
}

func TestClosingADirectorySessionKillsTheProcess(t *testing.T) {
	launcher := sandbox.NewDirectorySandbox()
	spec := sandbox.Spec{SignInToken: "signin-3", Workspace: t.TempDir(), Command: []string{"sleep", "30"}}
	session, err := launcher.StartInteractive(context.Background(), spec)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waited := make(chan struct{})
	go func() {
		_, _ = session.Wait()
		close(waited)
	}()

	_ = session.Close()

	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("the process outlived its session")
	}
}

func TestOutdatedSandboxImagesAreRemovedButNotTheCurrentOrOthers(t *testing.T) {
	engine := enginetest.NewFakeEngine(t)
	engine.StoredImages = []enginetest.StoredImage{
		{ID: "a", RepoTags: []string{"ghcr.io/acme/genwork-sandbox:2.3.0"}, Size: 1000},
		{ID: "b", RepoTags: []string{"ghcr.io/acme/genwork-sandbox:2.2.2"}, Size: 900},
		{ID: "c", RepoTags: []string{"ghcr.io/acme/genwork-sandbox:2.2.1"}, Size: 800},
		{ID: "d", RepoTags: []string{"ghcr.io/acme/genwork-sandbox:dev"}, Size: 700},
		{ID: "e", RepoTags: []string{"ghcr.io/acme/genwork-core:2.2.2"}, Size: 600},
		{ID: "f", RepoTags: []string{"registry.local:5000/team/genwork-sandbox:2.2.0"}, Size: 500},
	}
	engine.InUseImages = map[string]bool{"ghcr.io/acme/genwork-sandbox:2.2.1": true}
	launcher := sandbox.NewContainerSandbox(engine.Socket())

	freed, err := launcher.RemoveOutdatedImages(context.Background(), "ghcr.io/acme/genwork-sandbox:2.3.0")

	if err != nil || freed != 900 {
		t.Fatalf("freed = %d, err = %v", freed, err)
	}
	if len(engine.RemovedImages) != 1 || engine.RemovedImages[0] != "ghcr.io/acme/genwork-sandbox:2.2.2" {
		t.Errorf("removed = %v", engine.RemovedImages)
	}
}

func TestImageReferencesSplitAfterTheRegistryPort(t *testing.T) {
	for reference, want := range map[string][2]string{
		"ghcr.io/acme/genwork-sandbox:2.3.0":           {"ghcr.io/acme/genwork-sandbox", "2.3.0"},
		"registry.local:5000/team/genwork-sandbox:1.0": {"registry.local:5000/team/genwork-sandbox", "1.0"},
		"registry.local:5000/team/genwork-sandbox":     {"registry.local:5000/team/genwork-sandbox", ""},
	} {
		if repository, tag := sandbox.SplitImageReference(reference); repository != want[0] || tag != want[1] {
			t.Errorf("%s → %s, %s", reference, repository, tag)
		}
	}
}
