package runexec

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/skills"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const claimCandidateLimit = 50

// ClaimedRun is everything a runner needs to execute a run, secrets included. It is returned once, on claim; the copy
// stored with the run keeps only references (design §5).
type ClaimedRun struct {
	Run         runs.Run
	Bot         bots.Bot
	Account     accounts.AiAccount
	APIKey      string
	Skills      []skills.Skill
	MCPServers  []mcpservers.McpServer
	Instruction string
}

type RunClaimService struct {
	db          *pgxpool.Pool
	runners     runners.RunnerRepository
	runs        runs.RunRepository
	bots        bots.BotRepository
	accounts    accounts.AiAccountRepository
	skills      skills.SkillRepository
	accountsSvc *accounts.AiAccountService
	mcpServers  *mcpservers.McpServerService
	lifecycle   *runs.RunLifecycleService
	now         func() time.Time
}

func NewRunClaimService(db *pgxpool.Pool, accountService *accounts.AiAccountService, mcpServers *mcpservers.McpServerService, lifecycle *runs.RunLifecycleService, now func() time.Time) *RunClaimService {
	return &RunClaimService{db: db, accountsSvc: accountService, mcpServers: mcpServers, lifecycle: lifecycle, now: now}
}

// ClaimRun hands the oldest eligible queued run to the runner, or nil when there is none. Candidates are locked with
// SKIP LOCKED inside one transaction, so two runners never claim the same run.
func (service *RunClaimService) ClaimRun(ctx context.Context, runner runners.Runner, freeSlots int) (*ClaimedRun, error) {
	if freeSlots < 1 || runner.IsRevoked() {
		return nil, nil
	}
	var claimed *ClaimedRun
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		active, err := service.runs.CountActiveForRunner(ctx, tx, runner.ID)
		if err != nil || active >= runner.MaxConcurrency {
			return err
		}
		candidates, err := service.runs.LockQueued(ctx, tx, claimCandidateLimit)
		if err != nil {
			return err
		}
		sandboxed, err := service.sandboxedRunners(ctx, tx, runner)
		if err != nil {
			return err
		}
		for _, run := range candidates {
			picked, err := service.tryClaim(ctx, tx, runner, sandboxed, run)
			if err != nil {
				return err
			}
			if picked != nil {
				claimed = picked
				return nil
			}
		}
		return nil
	})
	return claimed, err
}

// sandboxedRunners lists the other online runners that use a sandbox: a directory-mode runner only takes work none of
// them can take.
func (service *RunClaimService) sandboxedRunners(ctx context.Context, tx pgx.Tx, runner runners.Runner) ([]runners.Runner, error) {
	if !runner.DirectoryMode {
		return nil, nil
	}
	all, err := service.runners.List(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := service.now()
	return slices.DeleteFunc(all, func(other runners.Runner) bool {
		return other.ID == runner.ID || other.DirectoryMode || !other.IsOnline(now)
	}), nil
}

func (service *RunClaimService) tryClaim(ctx context.Context, tx pgx.Tx, runner runners.Runner, sandboxed []runners.Runner, run runs.Run) (*ClaimedRun, error) {
	bot, err := service.bots.FindByID(ctx, tx, run.BotID)
	if errors.Is(err, bots.ErrBotNotFound) || (err == nil && !bot.IsEnabled()) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	account, err := service.accounts.FindByID(ctx, tx, bot.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Status == accounts.AccountStatusError {
		message := ""
		if account.LastError != nil {
			message = *account.LastError
		}
		return nil, service.lifecycle.Fail(ctx, tx, run, runs.FailureAccountError, map[string]any{"message": message, "account": account.Name})
	}
	if account.Status != accounts.AccountStatusReady || !IsEligible(runner, run, account) {
		return nil, nil
	}
	for _, other := range sandboxed {
		if IsEligible(other, run, account) {
			return nil, nil
		}
	}
	return service.claim(ctx, tx, runner, run, bot, account)
}

// IsEligible says whether a runner may take a run: it holds the subscription account or reports the provider's CLI,
// and has GitHub credentials when the run needs them.
func IsEligible(runner runners.Runner, run runs.Run, account accounts.AiAccount) bool {
	definition, found := accounts.FindProviderDefinition(account.Provider)
	if !found || runner.IsRevoked() {
		return false
	}
	if run.Kind.NeedsGithub() && !runner.HasGithubCredentials {
		return false
	}
	if account.Mode == accounts.AccountModeSubscription {
		return account.RunnerID != nil && *account.RunnerID == runner.ID && runner.ReportsTool(definition.CLI)
	}
	return runner.ReportsTool(definition.CLI)
}

func (service *RunClaimService) claim(ctx context.Context, tx pgx.Tx, runner runners.Runner, run runs.Run, bot bots.Bot, account accounts.AiAccount) (*ClaimedRun, error) {
	claimedRun := &ClaimedRun{Run: run, Bot: bot, Account: account}
	if account.Mode == accounts.AccountModeAPIKey {
		apiKey, err := service.accountsSvc.DecryptAPIKey(account)
		if err != nil {
			return nil, err
		}
		claimedRun.APIKey = apiKey
	}
	skillIDs, err := service.bots.ListSkillIDs(ctx, tx, bot.ID)
	if err != nil {
		return nil, err
	}
	if claimedRun.Skills, err = service.skills.ListByIDs(ctx, tx, skillIDs); err != nil {
		return nil, err
	}
	serverIDs, err := service.bots.ListMCPServerIDs(ctx, tx, bot.ID)
	if err != nil {
		return nil, err
	}
	if claimedRun.MCPServers, err = service.mcpServers.ResolveForRun(ctx, tx, serverIDs); err != nil {
		return nil, err
	}
	now := service.now()
	run.Spec.Skills = map[string]int{}
	for _, skill := range claimedRun.Skills {
		run.Spec.Skills[skill.Token] = skill.Version
	}
	if err := service.runs.UpdateSpec(ctx, tx, run.ID, run.Spec); err != nil {
		return nil, err
	}
	if err := service.runs.MarkClaimed(ctx, tx, run.ID, runner.ID, now.Add(bot.Timeout()), now); err != nil {
		return nil, err
	}
	run.State = runs.RunStateRunning
	claimedRun.Run = run
	return claimedRun, service.lifecycle.Post(ctx, tx, run, runs.EntryStatus, runs.StatusPayload{
		Code: "run_started", State: runs.RunStateRunning, Params: map[string]any{"runner": runner.Name},
	})
}
