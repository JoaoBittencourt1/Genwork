package runexec

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxEventsPerPost = 200

type EventInput struct {
	Sequence int64           `json:"sequence"`
	Type     runs.EntryType  `json:"type"`
	Payload  json.RawMessage `json:"payload"`
}

// EventSignals are the fields of a payload that change the run, not only the conversation.
type EventSignals struct {
	State     runs.RunState      `json:"state"`
	Reason    runs.FailureReason `json:"reason"`
	SessionID string             `json:"sessionId"`
	// AuthFailure says the provider refused the account's credentials.
	AuthFailure bool   `json:"authFailure"`
	Message     string `json:"message"`
}

type EventResult struct {
	Accepted int
	State    runs.RunState
}

type RunEventService struct {
	db        *pgxpool.Pool
	runs      runs.RunRepository
	entries   runs.EntryRepository
	bots      bots.BotRepository
	accounts  accounts.AiAccountRepository
	policy    RunAccessPolicy
	lifecycle *runs.RunLifecycleService
	now       func() time.Time
}

func NewRunEventService(db *pgxpool.Pool, lifecycle *runs.RunLifecycleService, now func() time.Time) *RunEventService {
	return &RunEventService{db: db, lifecycle: lifecycle, now: now}
}

// Ingest stores a batch of run events in order. A sequence already stored is ignored, so a retried post never appears
// twice; the answer tells the runner the run's state, so it learns of a cancellation.
func (service *RunEventService) Ingest(ctx context.Context, runner runners.Runner, runToken string, events []EventInput) (EventResult, error) {
	if len(events) == 0 || len(events) > maxEventsPerPost {
		return EventResult{}, httpapi.NewValidationError(map[string]string{"events": "invalid"})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Sequence < events[j].Sequence })
	var result EventResult
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		run, err := service.policy.FindRunOfRunner(ctx, tx, runner, runToken)
		if err != nil {
			return err
		}
		if !run.IsActive() {
			result.State = run.State
			return nil
		}
		for _, event := range events {
			if !event.Type.IsRunnerEntry() || event.Sequence < 1 || !json.Valid(event.Payload) {
				return httpapi.NewValidationError(map[string]string{"events": "invalid"})
			}
			added, err := service.entries.Append(ctx, tx, runs.NewEntry{
				ConversationID: run.ConversationID, RunID: &run.ID, RunSequence: &event.Sequence, Type: event.Type, Payload: event.Payload, At: service.now(),
			})
			if err != nil {
				return err
			}
			if !added {
				continue
			}
			result.Accepted++
			if err := service.applySignals(ctx, tx, &run, event); err != nil {
				return err
			}
		}
		latest, err := service.runs.FindByID(ctx, tx, run.ID)
		result.State = latest.State
		return err
	})
	return result, err
}

func (service *RunEventService) applySignals(ctx context.Context, tx pgx.Tx, run *runs.Run, event EventInput) error {
	var signals EventSignals
	if err := json.Unmarshal(event.Payload, &signals); err != nil {
		return nil
	}
	if signals.SessionID != "" {
		if err := service.runs.SetCLISession(ctx, tx, run.ID, signals.SessionID); err != nil {
			return err
		}
	}
	if !run.IsActive() {
		return nil
	}
	state, reason := nextStateOf(event.Type, signals)
	if state == "" {
		return nil
	}
	if signals.AuthFailure {
		if err := service.markAccountFailed(ctx, tx, *run, signals.Message); err != nil {
			return err
		}
		state, reason = runs.RunStateFailed, runs.FailureAccountError
	}
	var failure *runs.FailureReason
	if state == runs.RunStateFailed {
		failure = &reason
	}
	err := service.runs.UpdateState(ctx, tx, run.ID, state, failure, service.now())
	if errors.Is(err, runs.ErrRunNotActive) {
		return nil
	}
	if err == nil {
		run.State = state
	}
	return err
}

// nextStateOf maps an event to the state it asks for: results succeed, errors fail with a valid reason, and status events
// move between running and waiting.
func nextStateOf(entryType runs.EntryType, signals EventSignals) (runs.RunState, runs.FailureReason) {
	switch entryType {
	case runs.EntryResult:
		if signals.State == runs.RunStateFailed {
			return runs.RunStateFailed, validReasonOr(signals.Reason, runs.FailureGithubPostFailed)
		}
		return runs.RunStateSucceeded, ""
	case runs.EntryError:
		return runs.RunStateFailed, validReasonOr(signals.Reason, runs.FailureAdapterError)
	case runs.EntryStatus:
		if signals.State == runs.RunStateRunning || signals.State == runs.RunStateWaitingForInput {
			return signals.State, ""
		}
	}
	return "", ""
}

func validReasonOr(reason, fallback runs.FailureReason) runs.FailureReason {
	if reason.IsValid() {
		return reason
	}
	return fallback
}

// markAccountFailed records the provider's refusal on an API-key account; a subscription account reports its own state
// through its runner.
func (service *RunEventService) markAccountFailed(ctx context.Context, tx pgx.Tx, run runs.Run, message string) error {
	bot, err := service.bots.FindByID(ctx, tx, run.BotID)
	if errors.Is(err, bots.ErrBotNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	account, err := service.accounts.FindByID(ctx, tx, bot.AccountID)
	if err != nil || account.Mode != accounts.AccountModeAPIKey {
		return err
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return service.accounts.UpdateStatus(ctx, tx, account.ID, accounts.AccountStatusError, &message, service.now())
}
