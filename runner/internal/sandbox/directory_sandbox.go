package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DirectorySandbox runs the CLI directly on the host, in the run's workspace, without isolation. It exists only for
// development and only behind an explicit flag.
type DirectorySandbox struct {
	mutex     sync.Mutex
	processes map[string]*hostProcess
}

// hostProcess lets Stop wait for the process to be gone instead of sleeping through the whole grace period.
type hostProcess struct {
	process *os.Process
	done    chan struct{}
}

func NewDirectorySandbox() *DirectorySandbox {
	return &DirectorySandbox{processes: map[string]*hostProcess{}}
}

func (sandbox *DirectorySandbox) Run(ctx context.Context, spec Spec, output io.Writer) (Result, error) {
	if len(spec.Command) == 0 {
		return Result{}, errors.New("a sandboxed command needs at least a program")
	}
	command := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	command.Dir = spec.Workspace
	command.Env = hostEnvironment(spec)
	command.Stdout, command.Stderr = output, output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return signalGroup(command.Process, syscall.SIGTERM) }
	command.WaitDelay = stopGracePeriod
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start the command: %w", err)
	}
	entry := &hostProcess{process: command.Process, done: make(chan struct{})}
	sandbox.mutex.Lock()
	sandbox.processes[spec.RunToken] = entry
	sandbox.mutex.Unlock()
	defer func() {
		sandbox.mutex.Lock()
		delete(sandbox.processes, spec.RunToken)
		sandbox.mutex.Unlock()
		close(entry.done)
	}()
	err := command.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode()}, nil
	}
	if err != nil && ctx.Err() == nil {
		return Result{}, fmt.Errorf("run the command: %w", err)
	}
	return Result{}, nil
}

// hostEnvironment gives the process its own minimal environment plus the spec's, never the runner's secrets.
func hostEnvironment(spec Spec) []string {
	environment := []string{"HOME=" + spec.Workspace, "PATH=" + os.Getenv("PATH")}
	for key, value := range spec.Env {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func signalGroup(process *os.Process, signal syscall.Signal) error {
	if process == nil {
		return nil
	}
	if err := syscall.Kill(-process.Pid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// Stop ends the process group gracefully, and kills it when it ignores the signal.
func (sandbox *DirectorySandbox) Stop(ctx context.Context, runToken string) error {
	sandbox.mutex.Lock()
	entry, found := sandbox.processes[runToken]
	sandbox.mutex.Unlock()
	if !found {
		return nil
	}
	if err := signalGroup(entry.process, syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case <-entry.done:
		return nil
	case <-ctx.Done():
	case <-time.After(stopGracePeriod):
	}
	return signalGroup(entry.process, syscall.SIGKILL)
}

func (sandbox *DirectorySandbox) RemoveLeftovers(context.Context, map[string]bool) ([]string, error) {
	return nil, nil
}

// Describe reports the CLIs found on the host and says directory mode is on.
func (sandbox *DirectorySandbox) Describe(ctx context.Context, _ string) (Info, error) {
	info := Info{Engine: "directory mode", Tools: map[string]string{}, DirectoryMode: true}
	for _, tool := range []string{"claude", "codex", "cursor-agent", "grok"} {
		if path, err := exec.LookPath(tool); err == nil {
			info.Tools[tool] = hostToolVersion(ctx, path)
		}
	}
	return info, nil
}

func hostToolVersion(ctx context.Context, path string) string {
	output, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
}
