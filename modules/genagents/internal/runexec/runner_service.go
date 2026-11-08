package runexec

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RunnerService struct {
	db        *pgxpool.Pool
	runners   runners.RunnerRepository
	accounts  accounts.AiAccountRepository
	runs      runs.RunRepository
	lifecycle *runs.RunLifecycleService
	now       func() time.Time
}

func NewRunnerService(db *pgxpool.Pool, lifecycle *runs.RunLifecycleService, now func() time.Time) *RunnerService {
	return &RunnerService{db: db, lifecycle: lifecycle, now: now}
}

func (service *RunnerService) ListRunners(ctx context.Context) ([]runners.Runner, error) {
	return service.runners.List(ctx, service.db)
}

// CreateRunner returns the registration token once; only its hash is stored (rule 67).
func (service *RunnerService) CreateRunner(ctx context.Context, name string) (runners.Runner, string, error) {
	name = strings.TrimSpace(name)
	if length := utf8.RuneCountInString(name); length == 0 || length > runners.MaxRunnerNameLength {
		return runners.Runner{}, "", httpapi.NewValidationError(map[string]string{"name": "invalid"})
	}
	registrationToken := platform.NewBearerSecret()
	runner, err := service.runners.Create(ctx, service.db, name, platform.HashBearerSecret(registrationToken), service.now())
	return runner, registrationToken, err
}

// RevokeRunner rejects the runner's secret at once and fails the runs it holds.
func (service *RunnerService) RevokeRunner(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		runner, err := service.runners.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if err := service.runners.Revoke(ctx, tx, runner.ID, service.now()); err != nil {
			return err
		}
		held, err := service.runs.ListActiveForRunner(ctx, tx, runner.ID)
		if err != nil {
			return err
		}
		for _, run := range held {
			if err := service.lifecycle.Fail(ctx, tx, run, runs.FailureRunnerRevoked, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveRunner soft-deletes a runner that can no longer be used: already revoked, or never registered after its token expired.
// A runner that still holds subscription accounts stays, so those accounts do not lose the machine they point at.
func (service *RunnerService) RemoveRunner(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		runner, err := service.runners.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if !runner.CanBeRemoved(service.now()) {
			return runners.ErrRunnerNotRemovable
		}
		bound, err := service.accounts.ListSubscriptionByRunner(ctx, tx, runner.ID)
		if err != nil {
			return err
		}
		if len(bound) > 0 {
			names := make([]string, 0, len(bound))
			for _, account := range bound {
				names = append(names, account.Name)
			}
			return newRunnerInUseError(names)
		}
		return service.runners.SoftDelete(ctx, tx, runner.ID, service.now())
	})
}

func newRunnerInUseError(accountNames []string) *httpapi.APIError {
	return &httpapi.APIError{Status: http.StatusConflict, Code: "runner_in_use", Fields: map[string]string{"accounts": strings.Join(accountNames, ", ")}}
}

// ExchangeRegistration trades a one-time registration token for the runner's long-lived secret.
func (service *RunnerService) ExchangeRegistration(ctx context.Context, registrationToken string) (runners.Runner, string, error) {
	secret := platform.NewBearerSecret()
	runner, err := service.runners.ExchangeRegistration(ctx, service.db, platform.HashBearerSecret(registrationToken), platform.HashBearerSecret(secret), service.now())
	return runner, secret, err
}

// Authenticate finds the runner a secret belongs to; a revoked runner has no secret left, so it is refused.
func (service *RunnerService) Authenticate(ctx context.Context, secret string) (runners.Runner, error) {
	runner, err := service.runners.FindBySecretHash(ctx, service.db, platform.HashBearerSecret(secret))
	if err != nil {
		return runners.Runner{}, err
	}
	if runner.IsRevoked() {
		return runners.Runner{}, runners.ErrInvalidRunnerSecret
	}
	return runner, nil
}

type AccountAssignment struct {
	Token    string
	Provider accounts.Provider
}

// HeartbeatAnswer tells the runner which subscription accounts it holds profiles for, so it can delete the profile of an
// account that was removed, and whether an admin asked it to free space.
type HeartbeatAnswer struct {
	Assignments      []AccountAssignment
	CleanupRequested bool
}

// RecordHeartbeat stores what the runner reports, including a finished cleanup, and answers what it should do.
func (service *RunnerService) RecordHeartbeat(ctx context.Context, runner runners.Runner, heartbeat runners.Heartbeat) (HeartbeatAnswer, error) {
	if err := validateHeartbeat(heartbeat); err != nil {
		return HeartbeatAnswer{}, err
	}
	heartbeat.At = service.now()
	if err := service.runners.RecordHeartbeat(ctx, service.db, runner.ID, heartbeat); err != nil {
		return HeartbeatAnswer{}, err
	}
	recorded, err := service.runners.FindByID(ctx, service.db, runner.ID)
	if err != nil {
		return HeartbeatAnswer{}, err
	}
	bound, err := service.accounts.ListSubscriptionByRunner(ctx, service.db, runner.ID)
	if err != nil {
		return HeartbeatAnswer{}, err
	}
	answer := HeartbeatAnswer{Assignments: make([]AccountAssignment, 0, len(bound)), CleanupRequested: recorded.CleanupPending()}
	for _, account := range bound {
		answer.Assignments = append(answer.Assignments, AccountAssignment{Token: account.Token, Provider: account.Provider})
	}
	return answer, nil
}

func validateHeartbeat(heartbeat runners.Heartbeat) error {
	invalid := heartbeat.MaxConcurrency < 1 || heartbeat.MaxConcurrency > 64 || len(heartbeat.Tools) > 20 ||
		len(heartbeat.ContainerEngine) > 100 || len(heartbeat.SandboxImage) > 300 ||
		(heartbeat.CleanupFreedBytes != nil && *heartbeat.CleanupFreedBytes < 0)
	for name, version := range heartbeat.Tools {
		invalid = invalid || name == "" || len(name) > 50 || len(version) > 100
	}
	if invalid {
		return httpapi.NewValidationError(map[string]string{"heartbeat": "invalid"})
	}
	return nil
}

// ReportAccountStatus is how a runner tells genagents a sign-in of a subscription account it holds worked or failed.
func (service *RunnerService) ReportAccountStatus(ctx context.Context, runner runners.Runner, accountToken string, signedIn bool, message string) error {
	account, err := service.accounts.FindByToken(ctx, service.db, accountToken)
	if errors.Is(err, accounts.ErrAccountNotFound) || (err == nil && (account.RunnerID == nil || *account.RunnerID != runner.ID)) {
		return accounts.ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	status, lastError := accounts.AccountStatusReady, (*string)(nil)
	if !signedIn {
		status = accounts.AccountStatusError
		if len(message) > 500 {
			message = message[:500]
		}
		lastError = &message
	}
	return service.accounts.UpdateStatus(ctx, service.db, account.ID, status, lastError, service.now())
}
