package runexec

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const delegatedRefreshColumn = "conversations.delegated_refresh"

var ErrSessionRevoked = httpapi.NewAPIError(http.StatusConflict, "session_revoked")

// GenworkTokenClient is what the tokens need from core (rule 9).
type GenworkTokenClient interface {
	IssueBotAccessToken(ctx context.Context, botToken, clientSecret string) (corelink.BotAccessToken, error)
	RefreshSession(ctx context.Context, refreshToken string) (corelink.SignIn, error)
}

type GenworkToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

// GenworkTokenService issues the short-lived token a run's Genwork MCP uses: the bot member's for task and review runs,
// the admin's, through the chat's delegated session, for chat runs. It issues nothing for a run that has ended.
type GenworkTokenService struct {
	db            *pgxpool.Pool
	core          GenworkTokenClient
	cipher        *platform.SecretCipher
	botService    *bots.BotService
	bots          bots.BotRepository
	runs          runs.RunRepository
	conversations runs.ConversationRepository
	policy        RunAccessPolicy
	lifecycle     *runs.RunLifecycleService
	now           func() time.Time
}

func NewGenworkTokenService(db *pgxpool.Pool, core GenworkTokenClient, cipher *platform.SecretCipher, botService *bots.BotService, lifecycle *runs.RunLifecycleService, now func() time.Time) *GenworkTokenService {
	return &GenworkTokenService{db: db, core: core, cipher: cipher, botService: botService, lifecycle: lifecycle, now: now}
}

func DelegatedRefreshAssociatedData(conversationToken string) string {
	return delegatedRefreshColumn + "|" + conversationToken
}

func (service *GenworkTokenService) IssueToken(ctx context.Context, runner runners.Runner, runToken string) (GenworkToken, error) {
	run, err := service.policy.FindRunOfRunner(ctx, service.db, runner, runToken)
	if err != nil {
		return GenworkToken{}, err
	}
	if !run.IsActive() {
		return GenworkToken{}, runs.ErrRunNotActive
	}
	if run.Kind == runs.RunKindChat {
		return service.issueDelegatedToken(ctx, run)
	}
	bot, err := service.bots.FindByID(ctx, service.db, run.BotID)
	if err != nil {
		return GenworkToken{}, err
	}
	secret, err := service.botService.DecryptMemberSecret(bot)
	if err != nil {
		return GenworkToken{}, err
	}
	token, err := service.core.IssueBotAccessToken(ctx, bot.MemberToken, secret)
	if err != nil {
		return GenworkToken{}, err
	}
	return GenworkToken{AccessToken: token.AccessToken, ExpiresAt: token.ExpiresAt}, nil
}

// issueDelegatedToken rotates the chat's delegated refresh token and stores the new one. When core refuses it, the
// admin ended the session: the run fails and the conversation says to reopen the chat.
func (service *GenworkTokenService) issueDelegatedToken(ctx context.Context, run runs.Run) (GenworkToken, error) {
	var token GenworkToken
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		conversation, err := service.conversations.FindByID(ctx, tx, run.ConversationID)
		if err != nil {
			return err
		}
		refresh, err := service.cipher.Decrypt(conversation.DelegatedRefresh, DelegatedRefreshAssociatedData(conversation.Token))
		if err != nil {
			return err
		}
		signIn, err := service.core.RefreshSession(ctx, refresh)
		var refusal *corelink.RefusalError
		if errors.As(err, &refusal) {
			return ErrSessionRevoked
		}
		if err != nil {
			return err
		}
		ciphertext, err := service.cipher.Encrypt(signIn.RefreshToken, DelegatedRefreshAssociatedData(conversation.Token))
		if err != nil {
			return err
		}
		if err := service.conversations.UpdateDelegatedSession(ctx, tx, conversation.ID, signIn.Session, ciphertext, service.now()); err != nil {
			return err
		}
		token = GenworkToken{AccessToken: signIn.AccessToken, ExpiresAt: signIn.AccessTokenExpiresAt}
		return nil
	})
	if errors.Is(err, ErrSessionRevoked) {
		return GenworkToken{}, service.failRevokedSession(ctx, run)
	}
	return token, err
}

func (service *GenworkTokenService) failRevokedSession(ctx context.Context, run runs.Run) error {
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		return service.lifecycle.Fail(ctx, tx, run, runs.FailureSessionRevoked, nil)
	})
	if err != nil && !errors.Is(err, runs.ErrRunNotActive) {
		return err
	}
	return ErrSessionRevoked
}
