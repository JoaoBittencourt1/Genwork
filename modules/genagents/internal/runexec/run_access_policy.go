package runexec

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

// RunAccessPolicy is the one place that decides whether a runner may touch a run (rule 18): only the runner holding it
// can, and a run of another runner answers like a run that does not exist.
type RunAccessPolicy struct {
	runs runs.RunRepository
}

func (policy RunAccessPolicy) FindRunOfRunner(ctx context.Context, db platform.DBTX, runner runners.Runner, runToken string) (runs.Run, error) {
	run, err := policy.runs.FindByToken(ctx, db, runToken)
	if err != nil {
		return runs.Run{}, err
	}
	if run.RunnerID == nil || *run.RunnerID != runner.ID {
		return runs.Run{}, runs.ErrRunNotFound
	}
	return run, nil
}
