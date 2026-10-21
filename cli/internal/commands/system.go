package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"
)

type CommandRunner interface {
	Run(ctx context.Context, request RunRequest) error
}

type RunRequest struct {
	Name   string
	Args   []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Prompter interface {
	PromptLine(label string) (string, error)
	PromptPassword(label string) (string, error)
}

type ExecRunner struct{}

// Run interrupts instead of killing on cancellation so dev servers can shut down cleanly.
func (ExecRunner) Run(ctx context.Context, request RunRequest) error {
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Dir
	command.Stdin = request.Stdin
	command.Stdout = request.Stdout
	command.Stderr = request.Stderr
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = 10 * time.Second
	return command.Run()
}

type TerminalPrompter struct {
	input *bufio.Reader
}

func NewTerminalPrompter() *TerminalPrompter {
	return &TerminalPrompter{input: bufio.NewReader(os.Stdin)}
}

func (prompter *TerminalPrompter) PromptLine(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	line, err := prompter.input.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read %s: %w", strings.TrimSpace(strings.TrimSuffix(label, ":")), err)
	}
	return strings.TrimSpace(line), nil
}

func (prompter *TerminalPrompter) PromptPassword(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(password), nil
}
