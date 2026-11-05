package delegatedsessions

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrModuleNotEnabled = platform.NewValidationError(map[string]string{"module": "not_enabled"})

type DelegatedSessionService struct {
	db      *pgxpool.Pool
	auth    *auth.Service
	modules modules.ModuleRepository
}

func NewDelegatedSessionService(db *pgxpool.Pool, authService *auth.Service) *DelegatedSessionService {
	return &DelegatedSessionService{db: db, auth: authService}
}

// Grant starts a session of the caller that only the named, enabled module will hold.
func (service *DelegatedSessionService) Grant(ctx context.Context, caller platform.Principal, input GrantInput) (auth.SignIn, error) {
	found, err := service.modules.FindByNames(ctx, service.db, []string{input.Module})
	if err != nil {
		return auth.SignIn{}, err
	}
	if len(found) == 0 || !found[0].Enabled {
		return auth.SignIn{}, ErrModuleNotEnabled
	}
	var signIn auth.SignIn
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		signIn, err = service.auth.StartDelegatedSignIn(ctx, tx, caller.UserID, input.Module, input.Purpose)
		return err
	})
	return signIn, err
}
