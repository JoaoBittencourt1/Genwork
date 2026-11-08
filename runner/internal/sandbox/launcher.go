// Package sandbox runs a CLI invocation in isolation: in a disposable container by default, and directly on the host only
// in development, behind an explicit flag.
package sandbox

import (
	"context"
	"errors"
	"io"
)

const (
	// WorkspaceMount is where a run's workspace appears inside its sandbox.
	WorkspaceMount = "/workspace"
	// HomeMount is the sandbox's temporary home.
	HomeMount = "/home/sandbox"
	// RunLabel marks every container with the run it belongs to, so leftovers can be found after a crash.
	RunLabel = "genwork.run"
	// SignInLabel marks a sign-in container, so cleanup removes it without reporting a lost run.
	SignInLabel = "genwork.signin"
)

var ErrNoEngine = errors.New("no container engine is reachable: start Docker or Podman, or start the runner with --directory-mode for development without isolation")

// Limits are the resources a sandbox may use.
type Limits struct {
	CPUs   float64
	Memory int64
	PIDs   int64
}

// Profile is one account's sign-in directory, mounted at the path its CLI reads.
type Profile struct {
	Dir   string
	Mount string
}

// Spec is one sandboxed invocation. It carries no engine socket, no runner secret, and no GitHub credentials: only what
// the CLI needs to work in the workspace.
type Spec struct {
	RunToken string
	// SignInToken is set instead of RunToken for an account sign-in.
	SignInToken string
	Image       string
	Workspace   string
	Profile     *Profile
	Env         map[string]string
	Command     []string
	Limits      Limits
}

// Result tells how the sandboxed process ended.
type Result struct {
	ExitCode  int
	OOMKilled bool
}

// Info is what a runner reports about its sandbox in each heartbeat.
type Info struct {
	Engine        string
	ImageDigest   string
	Tools         map[string]string
	DirectoryMode bool
}

// Launcher is the boundary adapters and the executor never cross to reach a container engine (rule 9).
type Launcher interface {
	// Run starts the invocation, streams its output to the writer, and blocks until it ends.
	Run(ctx context.Context, spec Spec, output io.Writer) (Result, error)
	// Stop ends the invocation of a run: a graceful stop, then a kill.
	Stop(ctx context.Context, runToken string) error
	// RemoveLeftovers removes every sandbox of a run not in keep and returns those run tokens.
	RemoveLeftovers(ctx context.Context, keep map[string]bool) ([]string, error)
	Describe(ctx context.Context, image string) (Info, error)
}

// InteractiveSession is a sandboxed process the runner talks to: what its terminal prints and what it types into it.
type InteractiveSession interface {
	Output() io.Reader
	Input() io.Writer
	// Wait blocks until the process ends.
	Wait() (Result, error)
	// Close kills the process if it still runs and removes what it left.
	Close() error
}

// InteractiveLauncher starts a process with a terminal, for CLI sign-ins that print a URL and read a code.
type InteractiveLauncher interface {
	StartInteractive(ctx context.Context, spec Spec) (InteractiveSession, error)
}
