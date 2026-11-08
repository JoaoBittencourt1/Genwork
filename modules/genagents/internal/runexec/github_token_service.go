package runexec

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryTokenIssuer is what the relay needs from gentasks (rule 9).
type RepositoryTokenIssuer interface {
	IssueRepositoryToken(ctx context.Context, accessToken string, repositories []string) (gentasksclient.RepositoryToken, error)
}

// GithubToken is a short-lived GitHub token for one run's repositories.
type GithubToken struct {
	Token     string
	ExpiresAt time.Time
}

// NewGithubTokenUnavailableError tells the runner why the platform has no token for it, so it can fall back to its own.
func NewGithubTokenUnavailableError(reason string) *httpapi.APIError {
	return &httpapi.APIError{Status: http.StatusConflict, Code: "github_token_unavailable", Fields: map[string]string{"reason": reason}}
}

// GithubTokenService relays a GitHub token from gentasks to the runner of a task or review run. It asks with the bot's
// own Genwork token, so gentasks applies the bot's role, and keeps nothing.
type GithubTokenService struct {
	db       *pgxpool.Pool
	policy   RunAccessPolicy
	tokens   *GenworkTokenService
	gentasks RepositoryTokenIssuer
}

func NewGithubTokenService(db *pgxpool.Pool, tokens *GenworkTokenService, gentasks RepositoryTokenIssuer) *GithubTokenService {
	return &GithubTokenService{db: db, tokens: tokens, gentasks: gentasks}
}

func (service *GithubTokenService) IssueToken(ctx context.Context, runner runners.Runner, runToken string) (GithubToken, error) {
	run, err := service.policy.FindRunOfRunner(ctx, service.db, runner, runToken)
	if err != nil {
		return GithubToken{}, err
	}
	if !run.IsActive() {
		return GithubToken{}, runs.ErrRunNotActive
	}
	repositories := run.Spec.Repositories()
	if !run.Kind.NeedsGithub() || len(repositories) == 0 || service.gentasks == nil {
		return GithubToken{}, NewGithubTokenUnavailableError("not_applicable")
	}
	genworkToken, err := service.tokens.IssueToken(ctx, runner, runToken)
	if err != nil {
		return GithubToken{}, err
	}
	issued, err := service.gentasks.IssueRepositoryToken(ctx, genworkToken.AccessToken, repositories)
	var refusal *gentasksclient.RepositoryTokenRefusal
	switch {
	case errors.As(err, &refusal):
		slog.Warn("gentasks issued no GitHub token", "run", runToken, "reason", refusal.Reason)
		return GithubToken{}, NewGithubTokenUnavailableError(refusal.Reason)
	case err != nil:
		slog.Warn("gentasks issued no GitHub token", "run", runToken, "reason", "gentasks_unavailable", "err", err)
		return GithubToken{}, NewGithubTokenUnavailableError("gentasks_unavailable")
	}
	return GithubToken{Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
