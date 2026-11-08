package accounts

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const signInCodeColumn = "account_sign_ins.code"

// AccountSignInService drives the web-started sign-in of subscription accounts: a person starts and watches it, the
// runner holding the account runs the provider CLI's own sign-in and reports back. Provider credentials never reach
// genagents; only the code a person pastes passes through, encrypted, on its way to the CLI.
type AccountSignInService struct {
	db       *pgxpool.Pool
	cipher   *platform.SecretCipher
	accounts AiAccountRepository
	signIns  AccountSignInRepository
	now      func() time.Time
}

func NewAccountSignInService(db *pgxpool.Pool, cipher *platform.SecretCipher, now func() time.Time) *AccountSignInService {
	return &AccountSignInService{db: db, cipher: cipher, now: now}
}

func AssociatedDataForSignInCode(signInToken string) string {
	return signInCodeColumn + "|" + signInToken
}

// StartSignIn replaces the account's active sign-in with a new request for its runner.
func (service *AccountSignInService) StartSignIn(ctx context.Context, accountToken string) (AccountSignIn, error) {
	var started AccountSignIn
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		account, err := service.accounts.FindByToken(ctx, tx, accountToken)
		if err != nil {
			return err
		}
		if account.Mode != AccountModeSubscription || account.RunnerID == nil {
			return httpapi.NewValidationError(map[string]string{"mode": "not_applicable"})
		}
		now := service.now()
		if err := service.signIns.CancelActive(ctx, tx, account.ID, now); err != nil {
			return err
		}
		started, err = service.signIns.Create(ctx, tx, platform.NewPublicToken(), account.ID, now.Add(SignInLifetime), now)
		return err
	})
	return started, err
}

// SubmitCode stores the code a person copied from the provider's page until the runner takes it.
func (service *AccountSignInService) SubmitCode(ctx context.Context, accountToken, signInToken, code string) (AccountSignIn, error) {
	if err := service.ExpireOverdue(ctx); err != nil {
		return AccountSignIn{}, err
	}
	var submitted AccountSignIn
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		signIn, err := service.findOfAccount(ctx, tx, accountToken, signInToken)
		if err != nil {
			return err
		}
		if !signIn.State.IsActive() {
			return ErrSignInNotActive
		}
		if signIn.State != SignInAwaitingUser || !signIn.ExpectsCode {
			return ErrSignInNoCode
		}
		ciphertext, err := service.cipher.Encrypt(code, AssociatedDataForSignInCode(signIn.Token))
		if err != nil {
			return err
		}
		update := SignInUpdate{State: SignInCodeSubmitted, CodeCiphertext: ciphertext, At: service.now()}
		if err := service.signIns.Update(ctx, tx, signIn.ID, update); err != nil {
			return err
		}
		submitted, err = service.signIns.FindByToken(ctx, tx, signIn.AccountID, signIn.Token)
		return err
	})
	return submitted, err
}

// CancelSignIn ends an active sign-in; the account keeps the status it had, and the runner stops the CLI on its next poll.
func (service *AccountSignInService) CancelSignIn(ctx context.Context, accountToken, signInToken string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		signIn, err := service.findOfAccount(ctx, tx, accountToken, signInToken)
		if err != nil {
			return err
		}
		if !signIn.State.IsActive() {
			return ErrSignInNotActive
		}
		return service.signIns.Update(ctx, tx, signIn.ID, SignInUpdate{State: SignInCancelled, ClearCode: true, At: service.now()})
	})
}

func (service *AccountSignInService) findOfAccount(ctx context.Context, tx pgx.Tx, accountToken, signInToken string) (AccountSignIn, error) {
	account, err := service.accounts.FindByToken(ctx, tx, accountToken)
	if err != nil {
		return AccountSignIn{}, err
	}
	return service.signIns.FindByToken(ctx, tx, account.ID, signInToken)
}

// ExpireOverdue ends the sign-ins nobody finished in time and marks their accounts as failed.
func (service *AccountSignInService) ExpireOverdue(ctx context.Context) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error { return service.expireOverdue(ctx, tx) })
}

func (service *AccountSignInService) expireOverdue(ctx context.Context, tx pgx.Tx) error {
	now := service.now()
	accountIDs, err := service.signIns.ExpireOverdue(ctx, tx, now)
	if err != nil {
		return err
	}
	message := SignInExpiredError
	for _, accountID := range accountIDs {
		if err := service.accounts.UpdateStatus(ctx, tx, accountID, AccountStatusError, &message, now); err != nil {
			return err
		}
	}
	return nil
}

// ListForRunner returns the active sign-ins of the accounts a runner holds, with the pasted code of those waiting for it.
func (service *AccountSignInService) ListForRunner(ctx context.Context, runner runners.Runner) ([]RunnerSignIn, error) {
	if err := service.ExpireOverdue(ctx); err != nil {
		return nil, err
	}
	rows, err := service.signIns.ListActiveForRunner(ctx, service.db, runner.ID)
	if err != nil {
		return nil, err
	}
	listed := make([]RunnerSignIn, 0, len(rows))
	for _, row := range rows {
		entry := RunnerSignIn{Token: row.SignIn.Token, AccountToken: row.AccountToken, Provider: row.Provider, State: row.SignIn.State, ExpiresAt: row.SignIn.ExpiresAt}
		if row.SignIn.State == SignInCodeSubmitted && len(row.SignIn.CodeCiphertext) > 0 {
			if entry.Code, err = service.cipher.Decrypt(row.SignIn.CodeCiphertext, AssociatedDataForSignInCode(row.SignIn.Token)); err != nil {
				return nil, err
			}
		}
		listed = append(listed, entry)
	}
	return listed, nil
}

// ReportProgress records what the runner saw: the URL to open, that it took the code, or how the CLI ended.
func (service *AccountSignInService) ReportProgress(ctx context.Context, runner runners.Runner, signInToken string, progress SignInProgress) error {
	if err := service.ExpireOverdue(ctx); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		signIn, err := service.signIns.FindForRunner(ctx, tx, runner.ID, signInToken)
		if err != nil {
			return err
		}
		if !signIn.State.IsActive() {
			return ErrSignInNotActive
		}
		now := service.now()
		switch progress.State {
		case SignInAwaitingUser:
			return service.recordPrompt(ctx, tx, signIn, progress, now)
		case SignInVerifying:
			if signIn.State != SignInCodeSubmitted {
				return ErrSignInNoCode
			}
			return service.signIns.Update(ctx, tx, signIn.ID, SignInUpdate{State: SignInVerifying, ClearCode: true, At: now})
		case SignInSucceeded:
			if err := service.signIns.Update(ctx, tx, signIn.ID, SignInUpdate{State: SignInSucceeded, ClearCode: true, At: now}); err != nil {
				return err
			}
			return service.accounts.UpdateStatus(ctx, tx, signIn.AccountID, AccountStatusReady, nil, now)
		case SignInFailed:
			message := TruncateFailure(progress.Message)
			update := SignInUpdate{State: SignInFailed, ClearCode: true, Failure: &message, At: now}
			if err := service.signIns.Update(ctx, tx, signIn.ID, update); err != nil {
				return err
			}
			return service.accounts.UpdateStatus(ctx, tx, signIn.AccountID, AccountStatusError, &message, now)
		}
		return httpapi.NewValidationError(map[string]string{"state": "invalid"})
	})
}

func (service *AccountSignInService) recordPrompt(ctx context.Context, tx pgx.Tx, signIn AccountSignIn, progress SignInProgress, now time.Time) error {
	if signIn.State != SignInRequested && signIn.State != SignInAwaitingUser {
		return ErrSignInNotActive
	}
	fields := map[string]string{}
	if !IsValidSignInURL(progress.URL) {
		fields["url"] = "invalid"
	}
	if !IsValidUserCode(progress.UserCode) {
		fields["userCode"] = "invalid"
	}
	if len(fields) > 0 {
		return httpapi.NewValidationError(fields)
	}
	update := SignInUpdate{State: SignInAwaitingUser, SignInURL: &progress.URL, ExpectsCode: &progress.ExpectsCode, At: now}
	if progress.UserCode != "" {
		update.UserCode = &progress.UserCode
	}
	return service.signIns.Update(ctx, tx, signIn.ID, update)
}
