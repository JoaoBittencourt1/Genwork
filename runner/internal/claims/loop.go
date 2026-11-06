// Package claims asks genagents for work and starts each claimed run, never more at once than the runner's slots allow.
package claims

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
)

const retryBackoff = 5 * time.Second

// ErrRevoked means genagents refused the runner's secret: nothing more can be claimed.
var ErrRevoked = errors.New("the runner was revoked: its secret is no longer accepted")

type Claimer interface {
	Claim(ctx context.Context, freeSlots int) (*api.Run, error)
}

type Executor interface {
	Execute(ctx context.Context, run api.Run)
}

type Loop struct {
	claimer  Claimer
	executor Executor
	slots    int
	sleep    func(context.Context, time.Duration)

	mutex  sync.Mutex
	active map[string]bool
	group  sync.WaitGroup
}

func NewLoop(claimer Claimer, executor Executor, slots int) *Loop {
	return &Loop{claimer: claimer, executor: executor, slots: slots, sleep: sleepContext, active: map[string]bool{}}
}

func sleepContext(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}

// WithSleep replaces the wait between failed claims, for tests.
func (loop *Loop) WithSleep(sleep func(context.Context, time.Duration)) *Loop {
	loop.sleep = sleep
	return loop
}

// ActiveRuns lists the run tokens being executed, which the runner keeps when it removes leftovers.
func (loop *Loop) ActiveRuns() map[string]bool {
	loop.mutex.Lock()
	defer loop.mutex.Unlock()
	active := make(map[string]bool, len(loop.active))
	for token := range loop.active {
		active[token] = true
	}
	return active
}

func (loop *Loop) freeSlots() int {
	loop.mutex.Lock()
	defer loop.mutex.Unlock()
	return loop.slots - len(loop.active)
}

// Run claims until the context ends or the runner is revoked, then waits for the runs it started.
func (loop *Loop) Run(ctx context.Context) error {
	defer loop.group.Wait()
	for ctx.Err() == nil {
		free := loop.freeSlots()
		if free <= 0 {
			loop.sleep(ctx, time.Second)
			continue
		}
		run, err := loop.claimer.Claim(ctx, free)
		var statusErr *api.StatusError
		switch {
		case errors.As(err, &statusErr) && statusErr.IsRevoked():
			return ErrRevoked
		case err != nil && ctx.Err() == nil:
			slog.Warn("claim a run", "err", err)
			loop.sleep(ctx, retryBackoff)
		case run != nil:
			loop.start(ctx, *run)
		}
	}
	return nil
}

func (loop *Loop) start(ctx context.Context, run api.Run) {
	loop.mutex.Lock()
	loop.active[run.RunToken] = true
	loop.mutex.Unlock()
	loop.group.Add(1)
	go func() {
		defer loop.group.Done()
		defer func() {
			loop.mutex.Lock()
			delete(loop.active, run.RunToken)
			loop.mutex.Unlock()
		}()
		loop.executor.Execute(ctx, run)
	}()
}
