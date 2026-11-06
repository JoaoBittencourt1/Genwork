// Package workspace creates one directory per run, clones the repository into it on the host, and removes it with every
// secret file when the run ends. Git runs on the host with the runner's own token, handed over through a one-off
// credential helper and never written into the repository's configuration.
//
// The sandbox writes into the clone's .git, so the host treats everything in it as hostile: hooks, filters and the
// remote's URL are never trusted, only commands that talk to the Git server carry the token, and the credential helper
// answers only for the configured Git server's host.
package workspace

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	directoryMode = 0o700
	tokenVariable = "GENWORK_GIT_TOKEN"
	hostVariable  = "GENWORK_GIT_HOST"
	// credentialHelp answers only when git asks for the configured Git server's host, so a rewritten remote URL
	// (url.<base>.insteadOf, remote.origin.url) never receives the token.
	credentialHelp = `!f() { ok=; while IFS= read -r line; do [ "$line" = "host=$` + hostVariable + `" ] && ok=1; done; ` +
		`[ -n "$ok" ] && { echo username=x-access-token; echo "password=$` + tokenVariable + `"; }; }; f`
)

// untrustedConfig neutralizes repository configuration the sandbox may have written: a hook, a filesystem monitor, or
// a helper program would otherwise run on the host.
var untrustedConfig = []string{
	"-c", "core.hooksPath=/dev/null",
	"-c", "core.fsmonitor=false",
	"-c", "core.sshCommand=false",
	"-c", "core.gitProxy=false",
	"-c", "core.pager=cat",
	"-c", "core.editor=true",
	"-c", "diff.external=",
	"-c", "protocol.ext.allow=never",
}

type Manager struct {
	root       string
	gitBaseURL string
	gitHost    string
	token      string
}

func NewManager(root, gitBaseURL, githubToken string) *Manager {
	manager := &Manager{root: root, gitBaseURL: strings.TrimRight(gitBaseURL, "/"), token: githubToken}
	if parsed, err := url.Parse(manager.gitBaseURL); err == nil {
		manager.gitHost = parsed.Host
	}
	return manager
}

// Workspace is one run's directory: Dir is what the sandbox mounts, and Repositories are the clones inside it.
type Workspace struct {
	RunToken string
	Dir      string
	manager  *Manager
}

// Create makes the run's directory, empty and owner-only.
func (manager *Manager) Create(runToken string) (*Workspace, error) {
	if runToken == "" || strings.ContainsAny(runToken, `/\`) || runToken == "." || runToken == ".." {
		return nil, fmt.Errorf("invalid run token %q", runToken)
	}
	dir := filepath.Join(manager.root, runToken)
	if err := os.MkdirAll(dir, directoryMode); err != nil {
		return nil, fmt.Errorf("create the workspace: %w", err)
	}
	return &Workspace{RunToken: runToken, Dir: dir, manager: manager}, nil
}

// Remove deletes the workspace, including every file written into it.
func (workspace *Workspace) Remove() error {
	return os.RemoveAll(workspace.Dir)
}

// RemoveAll deletes every run workspace left on disk, except the ones in keep; it runs when the runner starts.
func (manager *Manager) RemoveAll(keep map[string]bool) error {
	entries, err := os.ReadDir(manager.root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}
	for _, entry := range entries {
		if !keep[entry.Name()] {
			if err := os.RemoveAll(filepath.Join(manager.root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// RepositoryURL is where a repository is cloned from: the configured Git server, then owner/name.
func (manager *Manager) RepositoryURL(repository string) string {
	return manager.gitBaseURL + "/" + repository + ".git"
}

// git runs on the host. Only an authenticated command gets the token and the credential helper; every command ignores
// the hooks and helpers the sandbox may have planted in the repository's configuration.
func (workspace *Workspace) git(ctx context.Context, directory string, authenticated bool, arguments ...string) (string, error) {
	prefix := append([]string{}, untrustedConfig...)
	environment := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1")
	if authenticated {
		prefix = append(prefix, "-c", "credential.helper=", "-c", "credential.helper="+credentialHelp)
		environment = append(environment, tokenVariable+"="+workspace.manager.token, hostVariable+"="+workspace.manager.gitHost)
	}
	command := exec.CommandContext(ctx, "git", append(prefix, arguments...)...)
	command.Dir = directory
	command.Env = environment
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", arguments[0], err, redact(output.String(), workspace.manager.token))
	}
	return strings.TrimSpace(output.String()), nil
}

func redact(text, token string) string {
	if token == "" {
		return text
	}
	return strings.ReplaceAll(text, token, "[redacted]")
}

// Clone fetches a repository into a directory of the workspace (the workspace itself when relative is empty), with a
// shallow history.
func (workspace *Workspace) Clone(ctx context.Context, repository, relative string) (string, error) {
	target := workspace.Dir
	if relative != "" {
		target = filepath.Join(workspace.Dir, relative)
	}
	if err := os.MkdirAll(target, directoryMode); err != nil {
		return "", fmt.Errorf("create the clone directory: %w", err)
	}
	if _, err := workspace.git(ctx, workspace.Dir, true, "clone", "--depth", "50", "--no-tags", workspace.manager.RepositoryURL(repository), target); err != nil {
		return "", err
	}
	return target, nil
}

// PrepareBranch checks out a fresh branch and sets the bot as the author of its commits.
func (workspace *Workspace) PrepareBranch(ctx context.Context, repositoryDir, branch, authorName, authorEmail string) error {
	steps := [][]string{
		{"config", "user.name", authorName},
		{"config", "user.email", authorEmail},
		{"checkout", "-b", branch},
	}
	for _, step := range steps {
		if _, err := workspace.git(ctx, repositoryDir, false, step...); err != nil {
			return err
		}
	}
	return workspace.ExcludeLocalFiles(repositoryDir, ".genwork/", ".claude/skills/", ".cursor/mcp.json", ".cursor/rules/", ".grok/mcp.json")
}

// ExcludeLocalFiles keeps the files the runner writes for the CLI out of every commit, without touching tracked files.
func (workspace *Workspace) ExcludeLocalFiles(repositoryDir string, patterns ...string) error {
	exclude := filepath.Join(repositoryDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), directoryMode); err != nil {
		return err
	}
	file, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open the exclude file: %w", err)
	}
	defer file.Close()
	_, err = file.WriteString(strings.Join(patterns, "\n") + "\n")
	return err
}

// CheckoutPullRequest fetches a pull request's head into a branch and checks it out.
func (workspace *Workspace) CheckoutPullRequest(ctx context.Context, repositoryDir string, number int) (string, error) {
	branch := "pr-" + strconv.Itoa(number)
	if _, err := workspace.git(ctx, repositoryDir, true, "fetch", "--depth", "50", "origin", fmt.Sprintf("pull/%d/head:%s", number, branch)); err != nil {
		return "", err
	}
	if _, err := workspace.git(ctx, repositoryDir, false, "checkout", branch); err != nil {
		return "", err
	}
	return branch, nil
}

// DefaultBranch is the branch the clone's origin points at.
func (workspace *Workspace) DefaultBranch(ctx context.Context, repositoryDir string) (string, error) {
	reference, err := workspace.git(ctx, repositoryDir, false, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(reference, "origin/"), nil
}

// CommitsAhead counts the commits of the current branch that the default branch does not have.
func (workspace *Workspace) CommitsAhead(ctx context.Context, repositoryDir string) (int, error) {
	count, err := workspace.git(ctx, repositoryDir, false, "rev-list", "--count", "origin/HEAD..HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(count)
}

// Push sends the branch to the repository's own URL from the host. It never reads the URL from the clone's
// configuration, which the sandbox can edit.
func (workspace *Workspace) Push(ctx context.Context, repository, repositoryDir, branch string) error {
	if branch == "" || strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid branch %q", branch)
	}
	reference := "refs/heads/" + branch
	_, err := workspace.git(ctx, repositoryDir, true, "push", workspace.manager.RepositoryURL(repository), reference+":"+reference)
	return err
}
