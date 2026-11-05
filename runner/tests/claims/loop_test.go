package claims_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/claims"
)

type scriptedClaimer struct {
	mutex    sync.Mutex
	runs     []*api.Run
	errs     []error
	requests []int
}

func (claimer *scriptedClaimer) Claim(ctx context.Context, freeSlots int) (*api.Run, error) {
	claimer.mutex.Lock()
	defer claimer.mutex.Unlock()
	claimer.requests = append(claimer.requests, freeSlots)
	if len(claimer.errs) > 0 {
		err := claimer.errs[0]
		claimer.errs = claimer.errs[1:]
		return nil, err
	}
	if len(claimer.runs) == 0 {
		return nil, context.Canceled
	}
	run := claimer.runs[0]
	claimer.runs = claimer.runs[1:]
	return run, nil
}

type blockingExecutor struct {
	started  atomic.Int32
	release  chan struct{}
	executed chan string
}

func (executor *blockingExecutor) Execute(_ context.Context, run api.Run) {
	executor.started.Add(1)
	executor.executed <- run.RunToken
	<-executor.release
}

func fastLoop(claimer claims.Claimer, executor claims.Executor, slots int) *claims.Loop {
	return claims.NewLoop(claimer, executor, slots).WithSleep(func(ctx context.Context, _ time.Duration) {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Millisecond):
		}
	})
}

func TestRunsAreStartedUpToTheNumberOfSlots(t *testing.T) {
	claimer := &scriptedClaimer{runs: []*api.Run{{RunToken: "one"}, {RunToken: "two"}, {RunToken: "three"}}}
	executor := &blockingExecutor{release: make(chan struct{}), executed: make(chan string, 3)}
	loop := fastLoop(claimer, executor, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- loop.Run(ctx) }()

	time.Sleep(150 * time.Millisecond)
	if started := executor.started.Load(); started != 2 {
		t.Errorf("started = %d with two slots", started)
	}
	if active := loop.ActiveRuns(); len(active) != 2 || !active["one"] || !active["two"] {
		t.Errorf("active = %v", active)
	}
	close(executor.release)
	if err := <-finished; err != nil {
		t.Errorf("run: %v", err)
	}
	claimer.mutex.Lock()
	defer claimer.mutex.Unlock()
	if claimer.requests[0] != 2 || claimer.requests[1] != 1 {
		t.Errorf("free slots asked = %v", claimer.requests)
	}
}

func TestRevokedRunnerStopsClaiming(t *testing.T) {
	claimer := &scriptedClaimer{errs: []error{&api.StatusError{Status: 401, Code: "invalid_runner_secret"}}}
	loop := fastLoop(claimer, &blockingExecutor{release: make(chan struct{}), executed: make(chan string, 1)}, 1)

	err := loop.Run(context.Background())

	if !errors.Is(err, claims.ErrRevoked) {
		t.Errorf("error = %v", err)
	}
}

func TestClaimErrorsAreRetriedAfterABackoff(t *testing.T) {
	claimer := &scriptedClaimer{errs: []error{errors.New("network down"), errors.New("still down")}, runs: []*api.Run{{RunToken: "one"}}}
	executor := &blockingExecutor{release: make(chan struct{}), executed: make(chan string, 1)}
	close(executor.release)
	loop := fastLoop(claimer, executor, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	if err := loop.Run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	select {
	case token := <-executor.executed:
		if token != "one" {
			t.Errorf("executed %s", token)
		}
	default:
		t.Error("the run was never executed after the retries")
	}
}
