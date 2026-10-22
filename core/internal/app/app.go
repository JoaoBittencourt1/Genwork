package app

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/capabilities"
	"github.com/JoaoBittencourt1/Genwork/core/internal/discovery"
	"github.com/JoaoBittencourt1/Genwork/core/internal/invites"
	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
	"github.com/JoaoBittencourt1/Genwork/core/internal/members"
	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RateLimits struct {
	Login       platform.RateLimit
	Refresh     platform.RateLimit
	Invitations platform.RateLimit
}

var DefaultRateLimits = RateLimits{
	Login:       platform.RateLimit{PerMinute: 10, Burst: 10},
	Refresh:     platform.RateLimit{PerMinute: 60, Burst: 30},
	Invitations: platform.RateLimit{PerMinute: 20, Burst: 10},
}

const maxConcurrentPasswordHashes = 4

type Settings struct {
	Version        string
	PublicURL      string
	InstanceName   string
	DefaultLocale  string
	SigningKey     auth.SigningKey
	TrustedProxies platform.ClientAddressResolver
	PasswordParams users.PasswordParams
	RateLimits     RateLimits
	ModuleSecret   string
}

type Dependencies struct {
	DB     *pgxpool.Pool
	Mailer mail.Sender
	Now    func() time.Time
}

// New is the composition root of the HTTP API.
func New(settings Settings, dependencies Dependencies) http.Handler {
	now := dependencies.Now
	db := dependencies.DB

	hasher := users.NewPasswordHasher(settings.PasswordParams, maxConcurrentPasswordHashes)
	issuer := auth.NewAccessTokenIssuer(settings.SigningKey, settings.PublicURL, now)
	sessionService := sessions.NewService(db, now)
	accessService := access.NewService(db)
	userService := users.NewService(hasher)
	authService := auth.NewService(db, hasher, issuer, sessionService, accessService, now)
	inviteService := invites.NewService(invites.Dependencies{
		DB: db, Users: userService, Access: accessService, Auth: authService, Mailer: dependencies.Mailer, Now: now,
	}, invites.Settings{
		PublicURL: settings.PublicURL, InstanceName: settings.InstanceName, DefaultLocale: settings.DefaultLocale,
	})
	moduleService := modules.NewModuleService(db, discovery.CoreAPIVersion, now)
	memberService := members.NewMemberService(db, accessService, now)

	limitBy := func(limit platform.RateLimit) auth.Middleware {
		limiter := platform.NewRateLimiter(limit, now)
		return func(next http.Handler) http.Handler {
			return platform.RateLimitByClientAddress(limiter, settings.TrustedProxies, next)
		}
	}

	router := platform.NewRouter(
		auth.NewAuthenticator(issuer, sessionService), accessService, modules.NewCredentialAuthenticator(settings.ModuleSecret),
	)
	router.HandleFunc("GET /healthz", platform.Public, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	discovery.NewHandler(settings.InstanceName, settings.Version).Register(router)
	auth.NewHandler(authService, sessionService, settings.SigningKey, limitBy(settings.RateLimits.Login), limitBy(settings.RateLimits.Refresh)).Register(router)
	users.NewHandler(db, userService, accessService).Register(router)
	access.NewHandler(accessService).Register(router)
	members.NewMemberHandler(memberService).Register(router)
	invites.NewHandler(inviteService, limitBy(settings.RateLimits.Invitations), now).Register(router)
	modules.NewModuleHandler(moduleService).Register(router)
	capabilities.NewCapabilitiesHandler(moduleService, accessService, settings.Version, discovery.CoreAPIVersion).Register(router)
	return router
}
