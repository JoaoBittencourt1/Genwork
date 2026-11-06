package runs

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
)

// StatusPayload is what status and error entries carry: a message key the web app localizes, with its parameters, so no
// user-facing copy is hardcoded here (rule 36).
type StatusPayload struct {
	Code   string         `json:"code"`
	State  RunState       `json:"state,omitempty"`
	Reason FailureReason  `json:"reason,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

const (
	CauseUser        = "user"
	CauseReassigned  = "reassigned"
	CauseBotDisabled = "bot_disabled"
	CauseSystem      = "system"
)

// RunLifecycleService is the one place a run changes state together with the conversation entry that tells people why.
type RunLifecycleService struct {
	runs    RunRepository
	entries EntryRepository
	now     func() time.Time
}

func NewRunLifecycleService(now func() time.Time) *RunLifecycleService {
	return &RunLifecycleService{now: now}
}

func (service *RunLifecycleService) Post(ctx context.Context, db platform.DBTX, run Run, entryType EntryType, payload any) error {
	_, err := service.entries.Append(ctx, db, NewEntry{ConversationID: run.ConversationID, RunID: &run.ID, Type: entryType, Payload: payload, At: service.now()})
	return err
}

// Queued records that a new run waits for a runner.
func (service *RunLifecycleService) Queued(ctx context.Context, db platform.DBTX, run Run) error {
	return service.Post(ctx, db, run, EntryStatus, StatusPayload{Code: "run_queued", State: RunStateQueued})
}

// Fail ends an active run with a reason and says so in the conversation.
func (service *RunLifecycleService) Fail(ctx context.Context, db platform.DBTX, run Run, reason FailureReason, params map[string]any) error {
	if err := service.runs.UpdateState(ctx, db, run.ID, RunStateFailed, &reason, service.now()); err != nil {
		return err
	}
	return service.Post(ctx, db, run, EntryError, StatusPayload{Code: "run_failed", State: RunStateFailed, Reason: reason, Params: params})
}

// Cancel ends an active run without a failure; the cause tells people who or what stopped it.
func (service *RunLifecycleService) Cancel(ctx context.Context, db platform.DBTX, run Run, cause string, params map[string]any) error {
	if err := service.runs.UpdateState(ctx, db, run.ID, RunStateCancelled, nil, service.now()); err != nil {
		return err
	}
	if params == nil {
		params = map[string]any{}
	}
	params["cause"] = cause
	return service.Post(ctx, db, run, EntryStatus, StatusPayload{Code: "run_cancelled", State: RunStateCancelled, Params: params})
}

func (service *RunLifecycleService) Succeed(ctx context.Context, db platform.DBTX, run Run) error {
	if err := service.runs.UpdateState(ctx, db, run.ID, RunStateSucceeded, nil, service.now()); err != nil {
		return err
	}
	return service.Post(ctx, db, run, EntryStatus, StatusPayload{Code: "run_succeeded", State: RunStateSucceeded})
}

// CancelActiveRunsForBot implements the bots' RunCanceller: a disabled or removed bot leaves no run behind.
func (service *RunLifecycleService) CancelActiveRunsForBot(ctx context.Context, db platform.DBTX, botID int64, cause string) error {
	active, err := service.runs.ListActiveForBot(ctx, db, botID)
	if err != nil {
		return err
	}
	for _, run := range active {
		if err := service.Cancel(ctx, db, run, cause, nil); err != nil {
			return err
		}
	}
	return nil
}
