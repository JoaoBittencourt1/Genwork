// Package sandboxtest is a scripted sandbox launcher for the runner's tests: it records every invocation and lets a test
// decide what the CLI inside would print and do.
package sandboxtest

import (
	"context"
	"io"
	"sync"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

// Script plays the CLI: it may write files in spec.Workspace, print output, and decide how the process ends.
type Script func(ctx context.Context, turn int, spec sandbox.Spec, output io.Writer) (sandbox.Result, error)

type FakeLauncher struct {
	mutex      sync.Mutex
	script     Script
	Specs      []sandbox.Spec
	Stopped    []string
	Leftovers  []string
	DescribeAs sandbox.Info
}

func NewFakeLauncher(script Script) *FakeLauncher {
	return &FakeLauncher{script: script, DescribeAs: sandbox.Info{Engine: "fake engine", ImageDigest: "sha256:fake", Tools: map[string]string{"claude": "1.0.0"}}}
}

func (launcher *FakeLauncher) Run(ctx context.Context, spec sandbox.Spec, output io.Writer) (sandbox.Result, error) {
	launcher.mutex.Lock()
	launcher.Specs = append(launcher.Specs, spec)
	turn := len(launcher.Specs)
	launcher.mutex.Unlock()
	return launcher.script(ctx, turn, spec, output)
}

func (launcher *FakeLauncher) Stop(_ context.Context, runToken string) error {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	launcher.Stopped = append(launcher.Stopped, runToken)
	return nil
}

func (launcher *FakeLauncher) RemoveLeftovers(_ context.Context, keep map[string]bool) ([]string, error) {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	var removed []string
	for _, runToken := range launcher.Leftovers {
		if !keep[runToken] {
			removed = append(removed, runToken)
		}
	}
	launcher.Leftovers = nil
	return removed, nil
}

func (launcher *FakeLauncher) Describe(context.Context, string) (sandbox.Info, error) {
	return launcher.DescribeAs, nil
}

func (launcher *FakeLauncher) Turns() int {
	launcher.mutex.Lock()
	defer launcher.mutex.Unlock()
	return len(launcher.Specs)
}
