package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
)

// StartInteractive runs the spec on the host with pipes instead of a terminal; like the rest of directory mode it is for
// development only.
func (sandbox *DirectorySandbox) StartInteractive(ctx context.Context, spec Spec) (InteractiveSession, error) {
	if len(spec.Command) == 0 {
		return nil, errors.New("a sandboxed command needs at least a program")
	}
	command := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	command.Dir = spec.Workspace
	command.Env = hostEnvironment(spec)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return signalGroup(command.Process, syscall.SIGKILL) }
	reader, writer := io.Pipe()
	command.Stdout, command.Stderr = writer, writer
	input, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open the command input: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start the command: %w", err)
	}
	return &hostSession{command: command, output: reader, outputWriter: writer, input: input}, nil
}

type hostSession struct {
	command      *exec.Cmd
	output       *io.PipeReader
	outputWriter *io.PipeWriter
	input        io.WriteCloser
}

func (session *hostSession) Output() io.Reader { return session.output }
func (session *hostSession) Input() io.Writer  { return session.input }

func (session *hostSession) Wait() (Result, error) {
	err := session.command.Wait()
	_ = session.outputWriter.Close()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode()}, nil
	}
	return Result{}, err
}

func (session *hostSession) Close() error {
	_ = session.input.Close()
	_ = session.output.Close()
	return signalGroup(session.command.Process, syscall.SIGKILL)
}
