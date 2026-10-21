package invites

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const emailDeliveryTimeout = 15 * time.Second

type Settings struct {
	PublicURL     string
	InstanceName  string
	DefaultLocale string
}

type Dependencies struct {
	DB     *pgxpool.Pool
	Users  *users.Service
	Access *access.Service
	Auth   *auth.Service
	Mailer mail.Sender
	Now    func() time.Time
}

type Service struct {
	Dependencies
	settings Settings
	invites  Repository
	userRepo users.Repository
}

func NewService(dependencies Dependencies, settings Settings) *Service {
	return &Service{Dependencies: dependencies, settings: settings}
}

type Delivery struct {
	Invite    Invite
	Link      string
	EmailSent bool
}

type CreateInput struct {
	Email     string
	RoleToken string
}

func (service *Service) Create(ctx context.Context, principal platform.Principal, input CreateInput) (Delivery, error) {
	email := users.NormalizeEmail(input.Email)
	if !users.IsValidEmail(email) {
		return Delivery{}, platform.NewValidationError(map[string]string{"email": "invalid"})
	}
	role, err := service.Access.FindAssignableRole(ctx, principal.RoleID, input.RoleToken)
	if err != nil {
		return Delivery{}, err
	}

	now := service.Now()
	secret := platform.NewBearerSecret()
	var invite Invite
	err = pgx.BeginFunc(ctx, service.DB, func(tx pgx.Tx) error {
		if err := service.invites.LockEmail(ctx, tx, email); err != nil {
			return err
		}
		registered, err := service.userRepo.ExistsByEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		if registered {
			return users.ErrEmailAlreadyRegistered
		}
		pending, err := service.invites.HasPendingForEmail(ctx, tx, email, now)
		if err != nil {
			return err
		}
		if pending {
			return ErrInviteAlreadyPending
		}
		invite, err = service.invites.Create(ctx, tx, NewInvite{
			Email:           email,
			RoleID:          role.ID,
			InvitedByUserID: principal.UserID,
			SecretHash:      platform.HashBearerSecret(secret),
			CreatedAt:       now,
			ExpiresAt:       now.Add(Lifetime),
		})
		return err
	})
	if err != nil {
		return Delivery{}, err
	}
	return service.deliver(ctx, invite, secret), nil
}

func (service *Service) List(ctx context.Context, statusFilter string) ([]Invite, error) {
	var wanted Status
	if statusFilter != "" {
		status, valid := ParseStatus(statusFilter)
		if !valid {
			return nil, platform.NewValidationError(map[string]string{"status": "invalid"})
		}
		wanted = status
	}
	all, err := service.invites.List(ctx, service.DB)
	if err != nil {
		return nil, err
	}
	now := service.Now()
	filtered := make([]Invite, 0, len(all))
	for _, invite := range all {
		if wanted == "" || invite.StatusAt(now) == wanted {
			filtered = append(filtered, invite)
		}
	}
	return filtered, nil
}

func (service *Service) Revoke(ctx context.Context, inviteToken string) error {
	invite, err := service.findByToken(ctx, inviteToken)
	if err != nil {
		return err
	}
	now := service.Now()
	if invite.StatusAt(now) != StatusPending {
		return ErrInviteNotPending
	}
	return service.invites.Revoke(ctx, service.DB, invite.ID, now)
}

// Resend replaces the secret, so the previous link stops working, and restarts the expiry window.
func (service *Service) Resend(ctx context.Context, inviteToken string) (Delivery, error) {
	invite, err := service.findByToken(ctx, inviteToken)
	if err != nil {
		return Delivery{}, err
	}
	now := service.Now()
	if status := invite.StatusAt(now); status != StatusPending && status != StatusExpired {
		return Delivery{}, ErrInviteNotPending
	}
	secret := platform.NewBearerSecret()
	invite.ExpiresAt = now.Add(Lifetime)
	if err := service.invites.ReplaceSecret(ctx, service.DB, invite.ID, platform.HashBearerSecret(secret), invite.ExpiresAt); err != nil {
		return Delivery{}, err
	}
	return service.deliver(ctx, invite, secret), nil
}

type Inspection struct {
	InstanceName string
	Email        string
	ExpiresAt    time.Time
}

func (service *Service) Inspect(ctx context.Context, secret string) (Inspection, error) {
	invite, err := service.invites.FindPendingBySecret(ctx, service.DB, platform.HashBearerSecret(secret), service.Now())
	if errors.Is(err, errNoInvite) {
		return Inspection{}, ErrInviteNotFound
	}
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{InstanceName: service.settings.InstanceName, Email: invite.Email, ExpiresAt: invite.ExpiresAt}, nil
}

type AcceptInput struct {
	Name     string
	Password string
	auth.ClientInput
}

// Accept creates the account, consumes the invite, and signs the person in as one transaction.
func (service *Service) Accept(ctx context.Context, secret string, input AcceptInput) (auth.SignIn, error) {
	if fields := auth.ValidateClientInput(input.ClientInput); len(fields) > 0 {
		return auth.SignIn{}, platform.NewValidationError(fields)
	}
	now := service.Now()
	var signIn auth.SignIn
	err := pgx.BeginFunc(ctx, service.DB, func(tx pgx.Tx) error {
		invite, err := service.invites.LockPendingBySecret(ctx, tx, platform.HashBearerSecret(secret), now)
		if errors.Is(err, errNoInvite) {
			return ErrInviteNotFound
		}
		if err != nil {
			return err
		}
		user, err := service.Users.Create(ctx, tx, users.CreateInput{
			Email: invite.Email, Name: input.Name, Password: input.Password, RoleID: invite.RoleID,
		})
		if err != nil {
			return err
		}
		if err := service.invites.MarkAccepted(ctx, tx, invite.ID, user.ID, now); err != nil {
			return err
		}
		signIn, err = service.Auth.StartSignIn(ctx, tx, user.ID, input.ClientInput)
		return err
	})
	return signIn, err
}

func (service *Service) findByToken(ctx context.Context, inviteToken string) (Invite, error) {
	invite, err := service.invites.FindByToken(ctx, service.DB, inviteToken)
	if errors.Is(err, errNoInvite) {
		return Invite{}, ErrInviteNotFound
	}
	return invite, err
}

// deliver never fails the request: the invite already exists and the link is returned for manual sharing.
func (service *Service) deliver(ctx context.Context, invite Invite, secret string) Delivery {
	link := service.settings.PublicURL + "/invite/" + secret
	message, err := mail.RenderInvite(mail.InviteEmail{
		Locale:       service.settings.DefaultLocale,
		To:           invite.Email,
		InstanceName: service.settings.InstanceName,
		InviterName:  invite.InviterName,
		Link:         link,
		ExpiresAt:    invite.ExpiresAt,
	})
	if err == nil {
		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), emailDeliveryTimeout)
		defer cancel()
		err = service.Mailer.Send(sendCtx, message)
	}
	if err != nil {
		slog.WarnContext(ctx, "invite e-mail not delivered", "invite", invite.Token, "err", err)
	}
	return Delivery{Invite: invite, Link: link, EmailSent: err == nil}
}
