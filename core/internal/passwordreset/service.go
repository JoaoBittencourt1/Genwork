package passwordreset

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
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
	Mailer mail.Sender
	Now    func() time.Time
}

type Service struct {
	Dependencies
	settings Settings
	resets   Repository
	userRepo users.Repository
	sessions sessions.Repository
}

func NewService(dependencies Dependencies, settings Settings) *Service {
	return &Service{Dependencies: dependencies, settings: settings}
}

// Request never reveals whether the e-mail belongs to an account: the e-mail goes out in the background, so the response time is the same.
func (service *Service) Request(ctx context.Context, rawEmail string) error {
	email := users.NormalizeEmail(rawEmail)
	if !users.IsValidEmail(email) {
		return platform.NewValidationError(map[string]string{"email": "invalid"})
	}
	user, err := service.userRepo.FindByEmail(ctx, service.DB, email)
	if errors.Is(err, users.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.IsDeactivated() {
		return nil
	}

	now := service.Now()
	secret := platform.NewBearerSecret()
	created := false
	err = pgx.BeginFunc(ctx, service.DB, func(tx pgx.Tx) error {
		if err := service.resets.LockUser(ctx, tx, user.ID); err != nil {
			return err
		}
		recent, err := service.resets.HasCreatedSince(ctx, tx, user.ID, now.Add(-RequestInterval))
		if err != nil || recent {
			return err
		}
		if err := service.resets.MarkPendingUsedForUser(ctx, tx, user.ID, now); err != nil {
			return err
		}
		created = true
		return service.resets.Create(ctx, tx, NewReset{
			UserID: user.ID, SecretHash: platform.HashBearerSecret(secret), CreatedAt: now, ExpiresAt: now.Add(Lifetime),
		})
	})
	if err != nil || !created {
		return err
	}
	go service.deliver(context.WithoutCancel(ctx), user, secret, now.Add(Lifetime))
	return nil
}

// Confirm changes the password, consumes the link, and signs the account out everywhere as one transaction.
func (service *Service) Confirm(ctx context.Context, secret, newPassword string) error {
	if err := users.ValidatePasswordLength(newPassword); err != nil {
		return err
	}
	now := service.Now()
	return pgx.BeginFunc(ctx, service.DB, func(tx pgx.Tx) error {
		reset, err := service.resets.LockPendingBySecret(ctx, tx, platform.HashBearerSecret(secret), now)
		if errors.Is(err, errNoReset) {
			return ErrResetNotFound
		}
		if err != nil {
			return err
		}
		if err := service.Users.ChangePassword(ctx, tx, reset.UserID, newPassword); err != nil {
			return err
		}
		if err := service.resets.MarkPendingUsedForUser(ctx, tx, reset.UserID, now); err != nil {
			return err
		}
		return service.sessions.RevokeAllForUser(ctx, tx, reset.UserID, now)
	})
}

func (service *Service) deliver(ctx context.Context, user users.User, secret string, expiresAt time.Time) {
	message, err := mail.RenderPasswordReset(mail.PasswordResetEmail{
		Locale:       service.settings.DefaultLocale,
		To:           user.Email,
		InstanceName: service.settings.InstanceName,
		Link:         service.settings.PublicURL + "/reset-password/" + secret,
		ExpiresAt:    expiresAt,
	})
	if err == nil {
		sendCtx, cancel := context.WithTimeout(ctx, emailDeliveryTimeout)
		defer cancel()
		err = service.Mailer.Send(sendCtx, message)
	}
	if err != nil {
		slog.WarnContext(ctx, "password reset e-mail not delivered", "user", user.Token, "err", err)
	}
}
