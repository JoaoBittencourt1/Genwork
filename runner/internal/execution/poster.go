package execution

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
)

const (
	postAttempts    = 4
	postBackoffBase = 500 * time.Millisecond
)

// RunAPI is what the executor needs from genagents (rule 9).
type RunAPI interface {
	PostEvents(ctx context.Context, runToken string, events []api.Event) (string, error)
	RunState(ctx context.Context, runToken string) (string, error)
	ListMessages(ctx context.Context, runToken string) ([]api.Message, error)
	AcknowledgeMessages(ctx context.Context, runToken string, upToSequence int64) error
	GenworkToken(ctx context.Context, runToken string) (api.GenworkToken, error)
	GithubToken(ctx context.Context, runToken string) (api.GithubToken, error)
}

// poster sends a run's events in order, each with the next sequence number, and retries a failed post with the same
// sequence, so genagents' idempotency keeps a retry from appearing twice.
type poster struct {
	api      RunAPI
	runToken string
	sleep    func(time.Duration)
	mutex    sync.Mutex
	sequence int64
	// state is the run's state as genagents last answered, so the executor can see a cancellation.
	state string
}

func newPoster(runAPI RunAPI, runToken string, sleep func(time.Duration)) *poster {
	return &poster{api: runAPI, runToken: runToken, sleep: sleep}
}

// Post sends one event; a post that keeps failing is logged and dropped, never fatal to the run.
func (poster *poster) Post(ctx context.Context, eventType string, payload map[string]any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		slog.Error("encode run event", "run", poster.runToken, "err", err)
		return
	}
	poster.mutex.Lock()
	defer poster.mutex.Unlock()
	poster.sequence++
	event := api.Event{Sequence: poster.sequence, Type: eventType, Payload: encoded}
	var lastErr error
	for attempt := 0; attempt < postAttempts; attempt++ {
		if attempt > 0 {
			poster.sleep(postBackoffBase << (attempt - 1))
		}
		state, err := poster.api.PostEvents(ctx, poster.runToken, []api.Event{event})
		if err == nil {
			poster.state = state
			return
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	slog.Warn("post run event", "run", poster.runToken, "sequence", event.Sequence, "err", lastErr)
}

// State is the run's state in the last answer genagents gave.
func (poster *poster) State() string {
	poster.mutex.Lock()
	defer poster.mutex.Unlock()
	return poster.state
}
