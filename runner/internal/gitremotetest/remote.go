// Package gitremotetest builds a local bare repository that stands in for GitHub in the runner's tests, served as
// file:// so no network is involved (rule 25).
package gitremotetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type Remote struct {
	// BaseURL is what GENWORK_GIT_BASE_URL points at; repositories live at BaseURL/<owner>/<name>.git.
	BaseURL string
	root    string
}

func run(t testing.TB, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Dev", "GIT_AUTHOR_EMAIL=dev@example.com", "GIT_COMMITTER_NAME=Dev", "GIT_COMMITTER_EMAIL=dev@example.com")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

// NewRemote creates acme/api with one commit on main.
func NewRemote(t testing.TB) *Remote {
	t.Helper()
	root := t.TempDir()
	remote := &Remote{BaseURL: "file://" + root, root: root}
	bare := filepath.Join(root, "acme", "api.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, bare, "init", "--bare", "--initial-branch=main")
	seed := t.TempDir()
	run(t, seed, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("# api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "add", ".")
	run(t, seed, "commit", "-m", "initial")
	run(t, seed, "push", bare, "main")
	run(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	return remote
}

func (remote *Remote) bare() string {
	return filepath.Join(remote.root, "acme", "api.git")
}

// Branches lists the branches of the remote.
func (remote *Remote) Branches(t testing.TB) []string {
	t.Helper()
	listed := run(t, remote.bare(), "for-each-ref", "--format=%(refname:short)", "refs/heads")
	return strings.Fields(listed)
}

// LastCommitMessage is the subject of the newest commit of a branch.
func (remote *Remote) LastCommitMessage(t testing.TB, branch string) string {
	t.Helper()
	return run(t, remote.bare(), "log", "-1", "--format=%s", branch)
}

// LastCommitAuthor is the author of the newest commit of a branch.
func (remote *Remote) LastCommitAuthor(t testing.TB, branch string) string {
	t.Helper()
	return run(t, remote.bare(), "log", "-1", "--format=%an <%ae>", branch)
}

// AddPullRequestHead publishes a commit as refs/pull/<number>/head, the way GitHub does for a pull request.
func (remote *Remote) AddPullRequestHead(t testing.TB, number int, file, content string) {
	t.Helper()
	work := t.TempDir()
	run(t, work, "clone", remote.bare(), ".")
	run(t, work, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(work, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, work, "add", ".")
	run(t, work, "commit", "-m", "feature")
	run(t, work, "push", remote.bare(), "feature:refs/pull/"+itoa(number)+"/head")
}

func itoa(number int) string {
	return strconv.Itoa(number)
}
