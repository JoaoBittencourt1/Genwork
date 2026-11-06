package members

import (
	"context"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BotMemberService struct {
	db          *pgxpool.Pool
	access      *access.Service
	members     *MemberService
	users       users.Repository
	credentials users.BotCredentialRepository
	now         func() time.Time
}

func NewBotMemberService(db *pgxpool.Pool, accessService *access.Service, memberService *MemberService, now func() time.Time) *BotMemberService {
	return &BotMemberService{db: db, access: accessService, members: memberService, now: now}
}

// Create adds the bot member and returns its client secret, which is stored only as a hash and never readable again.
func (service *BotMemberService) Create(ctx context.Context, caller platform.Principal, request CreateBotRequest) (Member, string, error) {
	if err := request.Validate(); err != nil {
		return Member{}, "", err
	}
	role, err := service.access.FindAssignableRole(ctx, caller.RoleID, request.Role)
	if err != nil {
		return Member{}, "", err
	}
	secret := platform.NewBearerSecret()
	var created users.User
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		created, err = service.users.CreateBot(ctx, tx, users.NewBot{Name: users.NormalizeName(request.Name), RoleID: role.ID})
		if err != nil {
			return err
		}
		_, err = service.credentials.Create(ctx, tx, created.ID, users.HashBotSecret(secret))
		return err
	})
	if err != nil {
		return Member{}, "", err
	}
	member, err := service.members.assembleOne(ctx, created)
	return member, secret, err
}

func (service *BotMemberService) Update(ctx context.Context, caller platform.Principal, botToken string, input BotUpdateInput) (Member, error) {
	var updated users.User
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		target, err := service.findManageableBot(ctx, tx, caller, botToken)
		if err != nil {
			return err
		}
		if input.Name != nil {
			if err := service.users.UpdateName(ctx, tx, target.ID, *input.Name); err != nil {
				return err
			}
		}
		if input.RoleToken != nil {
			if err := service.members.changeRole(ctx, tx, caller, target, *input.RoleToken); err != nil {
				return err
			}
		}
		if input.Status != nil {
			if err := service.members.changeStatus(ctx, tx, target, *input.Status); err != nil {
				return err
			}
		}
		updated, err = service.users.FindByID(ctx, tx, target.ID)
		return err
	})
	if err != nil {
		return Member{}, err
	}
	return service.members.assembleOne(ctx, updated)
}

// RotateSecret invalidates the previous secret and revokes every session of the bot at once.
func (service *BotMemberService) RotateSecret(ctx context.Context, caller platform.Principal, botToken string) (Member, string, error) {
	secret := platform.NewBearerSecret()
	var target users.User
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		var err error
		if target, err = service.findManageableBot(ctx, tx, caller, botToken); err != nil {
			return err
		}
		now := service.now()
		if err := service.credentials.Rotate(ctx, tx, target.ID, users.HashBotSecret(secret), now); err != nil {
			return err
		}
		return service.members.sessions.RevokeAllForUser(ctx, tx, target.ID, now)
	})
	if err != nil {
		return Member{}, "", err
	}
	member, err := service.members.assembleOne(ctx, target)
	return member, secret, err
}

func (service *BotMemberService) Remove(ctx context.Context, caller platform.Principal, botToken string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		target, err := service.findManageableBot(ctx, tx, caller, botToken)
		if err != nil {
			return err
		}
		now := service.now()
		if err := service.users.SoftDelete(ctx, tx, target.ID, now); err != nil {
			return err
		}
		return service.members.sessions.RevokeAllForUser(ctx, tx, target.ID, now)
	})
}

// findManageableBot treats a human like an unknown member, so the bot API never reveals or touches people.
func (service *BotMemberService) findManageableBot(ctx context.Context, db platform.DBTX, caller platform.Principal, botToken string) (users.User, error) {
	target, err := service.members.findManageableUser(ctx, db, caller, botToken)
	if err != nil {
		return users.User{}, err
	}
	if !target.IsBot() {
		return users.User{}, ErrMemberNotFound
	}
	return target, nil
}
