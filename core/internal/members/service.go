package members

import (
	"context"
	"errors"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MemberService struct {
	db           *pgxpool.Pool
	access       *access.Service
	users        users.Repository
	roles        access.RoleRepository
	restrictions access.ModuleRestrictionRepository
	sessions     sessions.Repository
	now          func() time.Time
}

func NewMemberService(db *pgxpool.Pool, accessService *access.Service, now func() time.Time) *MemberService {
	return &MemberService{db: db, access: accessService, now: now}
}

func (service *MemberService) List(ctx context.Context) ([]Member, error) {
	listedUsers, err := service.users.ListNotRemoved(ctx, service.db)
	if err != nil {
		return nil, err
	}
	return service.assembleMembers(ctx, service.db, listedUsers)
}

// Update applies every present field in one transaction, after the central guard.
func (service *MemberService) Update(ctx context.Context, caller platform.Principal, memberToken string, input UpdateInput) (Member, error) {
	var updated users.User
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		target, err := service.findManageableUser(ctx, tx, caller, memberToken)
		if err != nil {
			return err
		}
		if input.RoleToken != nil {
			if err := service.changeRole(ctx, tx, caller, target, *input.RoleToken); err != nil {
				return err
			}
		}
		if input.Status != nil {
			if err := service.changeStatus(ctx, tx, target, *input.Status); err != nil {
				return err
			}
		}
		if input.RestrictedModules != nil {
			if err := service.replaceRestrictedModules(ctx, tx, target, *input.RestrictedModules); err != nil {
				return err
			}
		}
		updated, err = service.users.FindByID(ctx, tx, target.ID)
		return err
	})
	if err != nil {
		return Member{}, err
	}
	assembled, err := service.assembleMembers(ctx, service.db, []users.User{updated})
	if err != nil {
		return Member{}, err
	}
	return assembled[0], nil
}

// Remove soft-deletes the member and revokes their sessions; their e-mail becomes free for a new invite.
func (service *MemberService) Remove(ctx context.Context, caller platform.Principal, memberToken string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		target, err := service.findManageableUser(ctx, tx, caller, memberToken)
		if err != nil {
			return err
		}
		now := service.now()
		if err := service.users.SoftDelete(ctx, tx, target.ID, now); err != nil {
			return err
		}
		return service.sessions.RevokeAllForUser(ctx, tx, target.ID, now)
	})
}

// findManageableUser is the one place that decides whether the caller may act on a member (rule 18).
func (service *MemberService) findManageableUser(ctx context.Context, db platform.DBTX, caller platform.Principal, memberToken string) (users.User, error) {
	target, err := service.users.FindByToken(ctx, db, memberToken)
	if errors.Is(err, users.ErrUserNotFound) {
		return users.User{}, ErrMemberNotFound
	}
	if err != nil {
		return users.User{}, err
	}
	if target.ID == caller.UserID {
		return users.User{}, ErrCannotManageSelf
	}
	callerRole, err := service.roles.FindByID(ctx, db, caller.RoleID)
	if err != nil {
		return users.User{}, err
	}
	targetRole, err := service.roles.FindByID(ctx, db, target.RoleID)
	if err != nil {
		return users.User{}, err
	}
	if !callerRole.Covers(targetRole) {
		return users.User{}, ErrMemberNotManageable
	}
	return target, nil
}

func (service *MemberService) changeRole(ctx context.Context, db platform.DBTX, caller platform.Principal, target users.User, roleToken string) error {
	role, err := service.access.FindAssignableRole(ctx, caller.RoleID, roleToken)
	if err != nil {
		return err
	}
	return service.users.UpdateRole(ctx, db, target.ID, role.ID)
}

func (service *MemberService) changeStatus(ctx context.Context, db platform.DBTX, target users.User, status MemberStatus) error {
	switch {
	case status == MemberStatusDeactivated && !target.IsDeactivated():
		now := service.now()
		if err := service.users.UpdateDeactivatedAt(ctx, db, target.ID, &now); err != nil {
			return err
		}
		return service.sessions.RevokeAllForUser(ctx, db, target.ID, now)
	case status == MemberStatusActive && target.IsDeactivated():
		return service.users.UpdateDeactivatedAt(ctx, db, target.ID, nil)
	}
	return nil
}

func (service *MemberService) replaceRestrictedModules(ctx context.Context, db platform.DBTX, target users.User, moduleNames []string) error {
	err := service.restrictions.Replace(ctx, db, target.ID, moduleNames, service.now())
	if errors.Is(err, access.ErrUnknownModule) {
		return ErrUnknownModule
	}
	return err
}

// assembleMembers loads roles and restrictions for all the users in two queries instead of one per user (rule 21).
func (service *MemberService) assembleMembers(ctx context.Context, db platform.DBTX, memberUsers []users.User) ([]Member, error) {
	roles, err := service.roles.List(ctx, db)
	if err != nil {
		return nil, err
	}
	rolesByID := make(map[int64]access.Role, len(roles))
	for _, role := range roles {
		rolesByID[role.ID] = role
	}
	userIDs := make([]int64, 0, len(memberUsers))
	for _, user := range memberUsers {
		userIDs = append(userIDs, user.ID)
	}
	restrictedByUser, err := service.restrictions.ListModuleNamesByUserIDs(ctx, db, userIDs)
	if err != nil {
		return nil, err
	}
	assembled := make([]Member, 0, len(memberUsers))
	for _, user := range memberUsers {
		assembled = append(assembled, Member{User: user, Role: rolesByID[user.RoleID], RestrictedModules: restrictedByUser[user.ID]})
	}
	return assembled, nil
}
