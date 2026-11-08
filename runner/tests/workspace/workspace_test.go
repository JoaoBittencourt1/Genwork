package workspace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/gitremotetest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/workspace"
)

const token = "ghp-super-secret-token"

func newWorkspace(t *testing.T) (*workspace.Workspace, *workspace.Manager, *gitremotetest.Remote) {
	t.Helper()
	remote := gitremotetest.NewRemote(t)
	manager := workspace.NewManager(t.TempDir(), remote.BaseURL, token)
	created, err := manager.Create("run-1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return created, manager, remote
}

func git(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestCloneAndBranchLeaveNoTokenInTheRepositoryConfig(t *testing.T) {
	created, _, _ := newWorkspace(t)
	ctx := context.Background()

	directory, err := created.Clone(ctx, "acme/api", "")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if err := created.PrepareBranch(ctx, directory, "bot/gen-1-abc123", "Dev Bot", "dev-bot@bots.genwork.invalid"); err != nil {
		t.Fatalf("branch: %v", err)
	}

	if directory != created.Dir {
		t.Errorf("clone directory = %s", directory)
	}
	if branch := git(t, directory, "rev-parse", "--abbrev-ref", "HEAD"); branch != "bot/gen-1-abc123" {
		t.Errorf("branch = %s", branch)
	}
	config, err := os.ReadFile(filepath.Join(directory, ".git", "config"))
	if err != nil || strings.Contains(string(config), token) || strings.Contains(string(config), "x-access-token") {
		t.Errorf("the git config holds the token: %s (%v)", config, err)
	}
	if name := git(t, directory, "config", "user.name"); name != "Dev Bot" {
		t.Errorf("author = %s", name)
	}
}

func TestLocalFilesAreExcludedFromCommits(t *testing.T) {
	created, _, _ := newWorkspace(t)
	ctx := context.Background()
	directory, _ := created.Clone(ctx, "acme/api", "")
	if err := created.PrepareBranch(ctx, directory, "bot/x", "Bot", "bot@bots.genwork.invalid"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{".genwork/mcp.json", ".claude/skills/a/SKILL.md", ".cursor/mcp.json"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(directory, file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, file), []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	status := git(t, directory, "status", "--porcelain")

	if status != "" {
		t.Errorf("runner files would be committed: %s", status)
	}
}

func TestCommitsAheadPushAndTheDefaultBranch(t *testing.T) {
	created, _, remote := newWorkspace(t)
	ctx := context.Background()
	directory, _ := created.Clone(ctx, "acme/api", "")
	if err := created.PrepareBranch(ctx, directory, "bot/gen-1-abc123", "Dev Bot", "dev-bot@bots.genwork.invalid"); err != nil {
		t.Fatal(err)
	}

	if ahead, err := created.CommitsAhead(ctx, directory); err != nil || ahead != 0 {
		t.Fatalf("ahead before = %d, err = %v", ahead, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "orders.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "GEN-1 speed up orders")

	ahead, err := created.CommitsAhead(ctx, directory)
	if err != nil || ahead != 1 {
		t.Fatalf("ahead = %d, err = %v", ahead, err)
	}
	if err := created.Push(ctx, "acme/api", directory, "bot/gen-1-abc123"); err != nil {
		t.Fatalf("push: %v", err)
	}
	if base, err := created.DefaultBranch(ctx, directory); err != nil || base != "main" {
		t.Errorf("default branch = %q, err = %v", base, err)
	}
	if !contains(remote.Branches(t), "bot/gen-1-abc123") || remote.LastCommitMessage(t, "bot/gen-1-abc123") != "GEN-1 speed up orders" {
		t.Errorf("remote branches = %v", remote.Branches(t))
	}
	if author := remote.LastCommitAuthor(t, "bot/gen-1-abc123"); !strings.Contains(author, "Dev Bot") {
		t.Errorf("author = %s", author)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestCheckoutPullRequestHead(t *testing.T) {
	created, _, remote := newWorkspace(t)
	remote.AddPullRequestHead(t, 12, "feature.txt", "from the pull request")
	ctx := context.Background()
	directory, err := created.Clone(ctx, "acme/api", "repos/acme__api")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	branch, err := created.CheckoutPullRequest(ctx, directory, 12)

	if err != nil || branch != "pr-12" {
		t.Fatalf("branch = %q, err = %v", branch, err)
	}
	if content, err := os.ReadFile(filepath.Join(directory, "feature.txt")); err != nil || string(content) != "from the pull request" {
		t.Errorf("checked out content = %q, err = %v", content, err)
	}
	if _, err := created.CheckoutPullRequest(ctx, directory, 99); err == nil {
		t.Error("a missing pull request was checked out")
	}
}

func TestErrorsNeverEchoTheToken(t *testing.T) {
	_, manager, _ := newWorkspace(t)
	created, _ := manager.Create("run-2")

	_, err := created.Clone(context.Background(), "acme/missing", "")

	if err == nil || strings.Contains(err.Error(), token) {
		t.Errorf("error = %v", err)
	}
}

func TestRemoveDeletesEverythingAndRemoveAllKeepsRunningRuns(t *testing.T) {
	manager := workspace.NewManager(t.TempDir(), "file:///x", token)
	one, _ := manager.Create("run-one")
	two, _ := manager.Create("run-two")
	if err := os.WriteFile(filepath.Join(one.Dir, "secret.json"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := one.Remove(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(one.Dir); !os.IsNotExist(err) {
		t.Error("the workspace is still there")
	}
	if err := manager.RemoveAll(map[string]bool{"run-two": true}); err != nil {
		t.Fatalf("remove all: %v", err)
	}
	if _, err := os.Stat(two.Dir); err != nil {
		t.Errorf("a kept workspace was removed: %v", err)
	}
	if err := manager.RemoveAll(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(two.Dir); !os.IsNotExist(err) {
		t.Error("RemoveAll left a workspace")
	}
	if err := workspace.NewManager(filepath.Join(t.TempDir(), "none"), "", "").RemoveAll(nil); err != nil {
		t.Errorf("removing from a missing root: %v", err)
	}
}

func TestInvalidRunTokensAreRefused(t *testing.T) {
	manager := workspace.NewManager(t.TempDir(), "", "")
	for _, runToken := range []string{"", "../escape", "a/b", ".", ".."} {
		if _, err := manager.Create(runToken); err == nil {
			t.Errorf("%q was accepted", runToken)
		}
	}
	if manager.RepositoryURL("acme/api") != "/acme/api.git" {
		t.Errorf("url = %s", manager.RepositoryURL("acme/api"))
	}
}

func commitFile(t *testing.T, directory, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, directory, "add", ".")
	git(t, directory, "commit", "-m", "change")
}

func preparedClone(t *testing.T) (*workspace.Workspace, string, *gitremotetest.Remote) {
	t.Helper()
	created, _, remote := newWorkspace(t)
	directory, err := created.Clone(context.Background(), "acme/api", "")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if err := created.PrepareBranch(context.Background(), directory, "bot/x", "Bot", "bot@bots.genwork.invalid"); err != nil {
		t.Fatal(err)
	}
	return created, directory, remote
}

func TestPushNeverRunsHooksTheSandboxPlanted(t *testing.T) {
	created, directory, remote := preparedClone(t)
	leak := filepath.Join(t.TempDir(), "leak")
	hook := "#!/bin/sh\nenv > " + leak + "\n"
	if err := os.WriteFile(filepath.Join(directory, ".git", "hooks", "pre-push"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, directory, "config", "core.hooksPath", filepath.Join(directory, ".git", "hooks"))
	commitFile(t, directory, "a.txt")

	if err := created.Push(context.Background(), "acme/api", directory, "bot/x"); err != nil {
		t.Fatalf("push: %v", err)
	}

	if _, err := os.Stat(leak); !os.IsNotExist(err) {
		t.Error("a hook planted in .git ran on the host")
	}
	if !contains(remote.Branches(t), "bot/x") {
		t.Errorf("remote branches = %v", remote.Branches(t))
	}
}

func TestPushIgnoresARemoteURLTheSandboxRewrote(t *testing.T) {
	created, directory, remote := preparedClone(t)
	attacker := t.TempDir()
	git(t, attacker, "init", "--bare")
	git(t, directory, "config", "remote.origin.url", attacker)
	git(t, directory, "config", "remote.origin.pushurl", attacker)
	commitFile(t, directory, "a.txt")

	if err := created.Push(context.Background(), "acme/api", directory, "bot/x"); err != nil {
		t.Fatalf("push: %v", err)
	}

	if !contains(remote.Branches(t), "bot/x") {
		t.Errorf("the branch did not reach the configured server: %v", remote.Branches(t))
	}
	if out := git(t, attacker, "branch", "--list"); out != "" {
		t.Errorf("the rewritten remote received the push: %s", out)
	}
}

func TestLocalCommandsDoNotReceiveTheToken(t *testing.T) {
	created, directory, _ := preparedClone(t)
	leak := filepath.Join(t.TempDir(), "leak")
	// fsmonitor would run on rev-list if the host honored the repository's configuration.
	script := filepath.Join(t.TempDir(), "monitor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nenv > "+leak+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, directory, "config", "core.fsmonitor", script)

	if _, err := created.CommitsAhead(context.Background(), directory); err != nil {
		t.Fatalf("commits ahead: %v", err)
	}
	if _, err := created.DefaultBranch(context.Background(), directory); err != nil {
		t.Fatalf("default branch: %v", err)
	}

	if data, err := os.ReadFile(leak); err == nil && strings.Contains(string(data), token) {
		t.Error("a local command ran the repository's fsmonitor with the token")
	}
}

func TestOnlyIdleWorkspacesAreRemoved(t *testing.T) {
	root := t.TempDir()
	manager := workspace.NewManager(root, "", "")
	idle, _ := manager.Create("idle-run")
	busy, _ := manager.Create("busy-run")
	if err := os.WriteFile(filepath.Join(idle.Dir, "big.bin"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(busy.Dir, "work.txt"), []byte("in progress"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, path := range []string{filepath.Join(idle.Dir, "big.bin"), idle.Dir} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}

	freed, err := manager.RemoveIdle(24*time.Hour, time.Now())

	if err != nil || freed != 4096 {
		t.Fatalf("freed = %d, err = %v", freed, err)
	}
	if _, err := os.Stat(idle.Dir); !os.IsNotExist(err) {
		t.Error("the idle workspace survived")
	}
	if _, err := os.Stat(busy.Dir); err != nil {
		t.Errorf("the busy workspace was removed: %v", err)
	}
}
