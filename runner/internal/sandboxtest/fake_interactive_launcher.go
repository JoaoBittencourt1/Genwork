package sandboxtest

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

// TerminalScript plays a CLI sign-in: it prints to the terminal, may read what is typed, and decides how it ends. It
// should return when ctx ends, as a killed process would.
type TerminalScript func(ctx context.Context, terminal io.Writer, typed *TypedInput) (sandbox.Result, error)

// TypedInput is what the runner types into the terminal, line by line (each line ends with a carriage return).
type TypedInput struct {
	lines chan string
}

// ReadLine waits for the next typed line, or returns false when ctx ends first.
func (input *TypedInput) ReadLine(ctx context.Context) (string, bool) {
	select {
	case line := <-input.lines:
		return line, true
	case <-ctx.Done():
		return "", false
	}
}

// FakeInteractiveLauncher runs a terminal script in place of a sign-in sandbox and records the specs it got.
type FakeInteractiveLauncher struct {
	mutex  sync.Mutex
	script TerminalScript
	Specs  []sandbox.Spec
	Closed int
}

func NewFakeInteractiveLauncher(script TerminalScript) *FakeInteractiveLauncher {
	return &FakeInteractiveLauncher{script: script}
}

func (launcher *FakeInteractiveLauncher) StartInteractive(ctx context.Context, spec sandbox.Spec) (sandbox.InteractiveSession, error) {
	launcher.mutex.Lock()
	launcher.Specs = append(launcher.Specs, spec)
	launcher.mutex.Unlock()
	reader, writer := io.Pipe()
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &fakeSession{launcher: launcher, output: reader, typed: &TypedInput{lines: make(chan string, 4)}, cancel: cancel, done: make(chan struct{})}
	go func() {
		session.result, session.err = launcher.script(sessionCtx, writer, session.typed)
		_ = writer.Close()
		close(session.done)
	}()
	return session, nil
}

// SpecsSnapshot returns the specs so far.
func (launcher *FakeInteractiveLauncher) SpecsSnapshot() []sandbox.Spec {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	return append([]sandbox.Spec(nil), launcher.Specs...)
}

// ClosedCount says how many sessions were closed.
func (launcher *FakeInteractiveLauncher) ClosedCount() int {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	return launcher.Closed
}

type fakeSession struct {
	launcher *FakeInteractiveLauncher
	output   *io.PipeReader
	typed    *TypedInput
	cancel   context.CancelFunc
	done     chan struct{}
	result   sandbox.Result
	err      error
}

func (session *fakeSession) Output() io.Reader { return session.output }

func (session *fakeSession) Input() io.Writer { return inputWriter{session.typed} }

func (session *fakeSession) Wait() (sandbox.Result, error) {
	<-session.done
	return session.result, session.err
}

func (session *fakeSession) Close() error {
	session.cancel()
	_ = session.output.Close()
	session.launcher.mutex.Lock()
	session.launcher.Closed++
	session.launcher.mutex.Unlock()
	return nil
}

type inputWriter struct {
	typed *TypedInput
}

func (writer inputWriter) Write(data []byte) (int, error) {
	writer.typed.lines <- strings.TrimSuffix(string(data), "\r")
	return len(data), nil
}
