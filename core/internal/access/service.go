package access

import (
	"context"
	"errors"
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db    *pgxpool.Pool
	roles RoleRepository
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (service *Service) FindRoleByID(ctx context.Context, id int64) (Role, error) {
	return service.roles.FindByID(ctx, service.db, id)
}

func (service *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return service.roles.List(ctx, service.db)
}

// FindAssignableRole resolves a role by token and checks that the assigner's role covers every permission it grants.
func (service *Service) FindAssignableRole(ctx context.Context, assignerRoleID int64, roleToken string) (Role, error) {
	target, err := service.roles.FindByToken(ctx, service.db, roleToken)
	if errors.Is(err, ErrRoleNotFound) {
		return Role{}, platform.NewValidationError(map[string]string{"role": "unknown"})
	}
	if err != nil {
		return Role{}, err
	}
	assigner, err := service.roles.FindByID(ctx, service.db, assignerRoleID)
	if err != nil {
		return Role{}, err
	}
	if !assigner.Covers(target) {
		return Role{}, ErrRoleNotAssignable
	}
	return target, nil
}

func (service *Service) RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, authenticated := platform.PrincipalFrom(r.Context())
		if !authenticated {
			platform.WriteError(w, r, platform.ErrInvalidToken)
			return
		}
		role, err := service.roles.FindByID(r.Context(), service.db, principal.RoleID)
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		if !role.Grants(permission) {
			platform.WriteError(w, r, platform.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
