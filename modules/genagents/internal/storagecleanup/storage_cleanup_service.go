// Package storagecleanup frees what genagents and its runners keep after it stopped being useful: ended sign-ins, used
// OAuth authorizations, and on each runner the sandbox images of other versions and leftover workspaces.
package storagecleanup

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// SignInRetention keeps a finished sign-in visible on its account for a week before it is removed.
	SignInRetention = 7 * 24 * time.Hour
	// AuthorizationRetention keeps a finished OAuth round trip a day, for troubleshooting a failed connection.
	AuthorizationRetention = 24 * time.Hour
)

// CleanupResult is what one cleanup removed in genagents and how many runners were asked to free space.
type CleanupResult struct {
	RemovedSignIns        int
	RemovedAuthorizations int
	RunnersAsked          int
}

type StorageCleanupService struct {
	db             *pgxpool.Pool
	signIns        accounts.AccountSignInRepository
	authorizations mcpservers.McpServerOAuthRepository
	runners        runners.RunnerRepository
	now            func() time.Time
}

func NewStorageCleanupService(db *pgxpool.Pool, now func() time.Time) *StorageCleanupService {
	return &StorageCleanupService{db: db, now: now}
}

// Clean removes genagents' expired rows and asks every runner in use to clean up on its next heartbeat.
func (service *StorageCleanupService) Clean(ctx context.Context) (CleanupResult, error) {
	var result CleanupResult
	now := service.now()
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		var err error
		if result.RemovedSignIns, err = service.signIns.DeleteEndedBefore(ctx, tx, now.Add(-SignInRetention)); err != nil {
			return err
		}
		if result.RemovedAuthorizations, err = service.authorizations.DeleteFinishedAuthorizationsBefore(ctx, tx, now.Add(-AuthorizationRetention)); err != nil {
			return err
		}
		result.RunnersAsked, err = service.runners.RequestCleanup(ctx, tx, now)
		return err
	})
	return result, err
}
