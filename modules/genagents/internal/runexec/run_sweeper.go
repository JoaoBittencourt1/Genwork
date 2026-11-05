package runexec

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SweepInterval = 30 * time.Second

// RunSweeper ends runs nobody is working on any more: those whose runner is lost, those past their timeout, and those
// that waited too long for a reply.
type RunSweeper struct {
	db        *pgxpool.Pool
	runs      runs.RunRepository
	runners   runners.RunnerRepository
	lifecycle *runs.RunLifecycleService
	now       func() time.Time
}

func NewRunSweeper(db *pgxpool.Pool, lifecycle *runs.RunLifecycleService, now func() time.Time) *RunSweeper {
	return &RunSweeper{db: db, lifecycle: lifecycle, now: now}
}

// SweepOnce ends every run that must end now and returns how many it ended.
func (sweeper *RunSweeper) SweepOnce(ctx context.Context) (int, error) {
	ended := 0
	err := pgx.BeginFunc(ctx, sweeper.db, func(tx pgx.Tx) error {
		held, err := sweeper.runs.ListToSweep(ctx, tx)
		if err != nil {
			return err
		}
		now := sweeper.now()
		for _, run := range held {
			reason, due, err := sweeper.reasonToEnd(ctx, tx, run, now)
			if err != nil {
				return err
			}
			if !due {
				continue
			}
			if err := sweeper.lifecycle.Fail(ctx, tx, run, reason, nil); err != nil {
				return err
			}
			ended++
		}
		return nil
	})
	return ended, err
}

func (sweeper *RunSweeper) reasonToEnd(ctx context.Context, tx pgx.Tx, run runs.Run, now time.Time) (runs.FailureReason, bool, error) {
	if run.RunnerID != nil {
		runner, err := sweeper.runners.FindByID(ctx, tx, *run.RunnerID)
		if err != nil && !errors.Is(err, runners.ErrRunnerNotFound) {
			return "", false, err
		}
		if errors.Is(err, runners.ErrRunnerNotFound) || (!runner.IsRevoked() && runner.IsLost(now)) {
			return runs.FailureRunnerLost, true, nil
		}
	}
	if run.State == runs.RunStateWaitingForInput && run.WaitingSince != nil && now.Sub(*run.WaitingSince) >= runs.NoReplyTimeout {
		return runs.FailureNoReply, true, nil
	}
	if run.State == runs.RunStateRunning && run.TimeoutAt != nil && !now.Before(*run.TimeoutAt) {
		return runs.FailureTimeout, true, nil
	}
	return "", false, nil
}

// Run sweeps on a fixed interval until the context ends; one failed sweep is logged and the next one tries again.
func (sweeper *RunSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := sweeper.SweepOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Error("sweep runs", "err", err)
			}
		}
	}
}
