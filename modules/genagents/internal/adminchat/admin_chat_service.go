// Package adminchat is the chat only admins have: a general-purpose AI that acts as the admin through the Genwork MCP,
// with a delegated session the admin grants genagents (core-delegated-sessions).
package adminchat

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/bots"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runexec"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	moduleName       = "genagents"
	delegatedPurpose = "Admin chat"
	MaxTitleLength   = 100
	defaultTitle     = "New chat"
)

// ErrChatNotFound answers for a chat that does not exist and for another admin's chat alike, so chats cannot be probed.
var ErrChatNotFound = httpapi.NewAPIError(http.StatusNotFound, "chat_not_found")

// DelegatedSessionClient is what the chat needs from core (rule 9).
type DelegatedSessionClient interface {
	GrantDelegatedSession(ctx context.Context, accessToken, module, purpose string) (corelink.SignIn, error)
	RefreshSession(ctx context.Context, refreshToken string) (corelink.SignIn, error)
	EndSession(ctx context.Context, accessToken string) error
}

type Caller struct {
	UserToken   string
	AccessToken string
}

type ChatView struct {
	Conversation runs.Conversation
	BotName      string
	Entries      []runs.Entry
	LastSequence int64
	ActiveRun    *runs.Run
}

type AdminChatService struct {
	db            *pgxpool.Pool
	core          DelegatedSessionClient
	cipher        *platform.SecretCipher
	bots          bots.BotRepository
	conversations runs.ConversationRepository
	runs          runs.RunRepository
	entries       runs.EntryRepository
	lifecycle     *runs.RunLifecycleService
	now           func() time.Time
}

func NewAdminChatService(db *pgxpool.Pool, core DelegatedSessionClient, cipher *platform.SecretCipher, lifecycle *runs.RunLifecycleService, now func() time.Time) *AdminChatService {
	return &AdminChatService{db: db, core: core, cipher: cipher, lifecycle: lifecycle, now: now}
}

// StartChat grants genagents a delegated session of the admin and opens a chat with an enabled assistant bot.
func (service *AdminChatService) StartChat(ctx context.Context, caller Caller, botToken, title string) (runs.Conversation, error) {
	title = normalizeTitle(title)
	if !isValidTitle(title) {
		return runs.Conversation{}, httpapi.NewValidationError(map[string]string{"title": "invalid"})
	}
	bot, err := service.findAssistant(ctx, botToken)
	if err != nil {
		return runs.Conversation{}, err
	}
	signIn, err := service.core.GrantDelegatedSession(ctx, caller.AccessToken, moduleName, delegatedPurpose)
	if err != nil {
		return runs.Conversation{}, relayCoreError(err)
	}
	var conversation runs.Conversation
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		token := platform.NewPublicToken()
		ciphertext, err := service.cipher.Encrypt(signIn.RefreshToken, runexec.DelegatedRefreshAssociatedData(token))
		if err != nil {
			return err
		}
		conversation, err = service.createChat(ctx, tx, token, caller.UserToken, title, bot.ID, signIn.Session, ciphertext)
		return err
	})
	if err != nil {
		_ = service.core.EndSession(ctx, signIn.AccessToken)
	}
	return conversation, err
}

func (service *AdminChatService) createChat(ctx context.Context, tx pgx.Tx, token, owner, title string, botID int64, session string, ciphertext []byte) (runs.Conversation, error) {
	_, err := tx.Exec(ctx, `
		INSERT INTO conversations (token, kind, owner_user_token, title, bot_id, delegated_session, delegated_refresh_ciphertext, created_at, updated_at)
		VALUES ($1, 'admin_chat', $2, $3, $4, $5, $6, $7, $7)`, token, owner, title, botID, session, ciphertext, service.now())
	if err != nil {
		return runs.Conversation{}, err
	}
	return service.conversations.FindByToken(ctx, tx, token)
}

func (service *AdminChatService) findAssistant(ctx context.Context, botToken string) (bots.Bot, error) {
	bot, err := service.bots.FindByToken(ctx, service.db, botToken)
	if errors.Is(err, bots.ErrBotNotFound) || (err == nil && (bot.Kind != bots.BotKindAssistant || !bot.IsEnabled())) {
		return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "not_an_assistant"})
	}
	return bot, err
}

func (service *AdminChatService) ListChats(ctx context.Context, caller Caller) ([]runs.Conversation, error) {
	return service.conversations.ListAdminChats(ctx, service.db, caller.UserToken)
}

// findOwnChat is the one place that decides whose chat this is (rule 18): another admin's chat does not exist.
func (service *AdminChatService) findOwnChat(ctx context.Context, db platform.DBTX, caller Caller, token string) (runs.Conversation, error) {
	conversation, err := service.conversations.FindByToken(ctx, db, token)
	if errors.Is(err, runs.ErrConversationNotFound) {
		return runs.Conversation{}, ErrChatNotFound
	}
	if err != nil {
		return runs.Conversation{}, err
	}
	if conversation.Kind != runs.ConversationKindAdminChat || conversation.OwnerUserToken == nil || *conversation.OwnerUserToken != caller.UserToken {
		return runs.Conversation{}, ErrChatNotFound
	}
	return conversation, nil
}

func (service *AdminChatService) ReadChat(ctx context.Context, caller Caller, token string, afterSequence int64) (ChatView, error) {
	conversation, err := service.findOwnChat(ctx, service.db, caller, token)
	if err != nil {
		return ChatView{}, err
	}
	entries, err := service.entries.ListAfter(ctx, service.db, conversation.ID, afterSequence)
	if err != nil {
		return ChatView{}, err
	}
	last, err := service.entries.LastSequence(ctx, service.db, conversation.ID)
	if err != nil {
		return ChatView{}, err
	}
	view := ChatView{Conversation: conversation, Entries: entries, LastSequence: last}
	if conversation.BotID != nil {
		bot, err := service.bots.FindByID(ctx, service.db, *conversation.BotID)
		if err != nil && !errors.Is(err, bots.ErrBotNotFound) {
			return ChatView{}, err
		}
		view.BotName = bot.Name
	}
	active, err := service.runs.ListActiveForConversation(ctx, service.db, conversation.ID)
	if err != nil {
		return ChatView{}, err
	}
	if len(active) > 0 {
		view.ActiveRun = &active[0]
	}
	return view, nil
}

func (service *AdminChatService) RenameChat(ctx context.Context, caller Caller, token, title string) error {
	title = normalizeTitle(title)
	if !isValidTitle(title) {
		return httpapi.NewValidationError(map[string]string{"title": "invalid"})
	}
	conversation, err := service.findOwnChat(ctx, service.db, caller, token)
	if err != nil {
		return err
	}
	return service.conversations.UpdateTitle(ctx, service.db, conversation.ID, title, service.now())
}

// DeleteChat cancels the chat's runs, removes it, and revokes its delegated session in core.
func (service *AdminChatService) DeleteChat(ctx context.Context, caller Caller, token string) error {
	var refresh string
	var conversation runs.Conversation
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		var err error
		if conversation, err = service.findOwnChat(ctx, tx, caller, token); err != nil {
			return err
		}
		refresh = service.decryptRefresh(conversation)
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil {
			return err
		}
		for _, run := range active {
			if err := service.lifecycle.Cancel(ctx, tx, run, runs.CauseUser, nil); err != nil {
				return err
			}
		}
		return service.conversations.SoftDelete(ctx, tx, conversation.ID, service.now())
	})
	if err != nil {
		return err
	}
	service.revokeSession(ctx, refresh)
	return nil
}

// revokeSession ends the delegated session; a session that is already gone needs nothing more.
func (service *AdminChatService) revokeSession(ctx context.Context, refresh string) {
	if refresh == "" {
		return
	}
	signIn, err := service.core.RefreshSession(ctx, refresh)
	if err != nil {
		return
	}
	_ = service.core.EndSession(ctx, signIn.AccessToken)
}

func (service *AdminChatService) decryptRefresh(conversation runs.Conversation) string {
	if len(conversation.DelegatedRefresh) == 0 {
		return ""
	}
	refresh, err := service.cipher.Decrypt(conversation.DelegatedRefresh, runexec.DelegatedRefreshAssociatedData(conversation.Token))
	if err != nil {
		return ""
	}
	return refresh
}

// ReopenChat grants a new delegated session after the previous one ended, so the chat can take turns again.
func (service *AdminChatService) ReopenChat(ctx context.Context, caller Caller, token string) error {
	conversation, err := service.findOwnChat(ctx, service.db, caller, token)
	if err != nil {
		return err
	}
	signIn, err := service.core.GrantDelegatedSession(ctx, caller.AccessToken, moduleName, delegatedPurpose)
	if err != nil {
		return relayCoreError(err)
	}
	ciphertext, err := service.cipher.Encrypt(signIn.RefreshToken, runexec.DelegatedRefreshAssociatedData(conversation.Token))
	if err != nil {
		return err
	}
	return service.conversations.UpdateDelegatedSession(ctx, service.db, conversation.ID, signIn.Session, ciphertext, service.now())
}

// PostMessage records the admin's message and queues one chat run for it, resuming the chat's CLI session. While a run
// is still active, its runner picks the message up at its next turn instead.
func (service *AdminChatService) PostMessage(ctx context.Context, caller Caller, token, text string) error {
	text = strings.TrimSpace(text)
	if !runs.IsValidMessage(text) {
		return runs.ErrMessageInvalid
	}
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		conversation, err := service.findOwnChat(ctx, tx, caller, token)
		if err != nil {
			return err
		}
		bot, err := service.findChatBot(ctx, tx, conversation)
		if err != nil {
			return err
		}
		author := caller.UserToken
		if _, err := service.entries.Append(ctx, tx, runs.NewEntry{
			ConversationID: conversation.ID, Type: runs.EntryHumanMessage, AuthorToken: &author, Payload: map[string]any{"text": text}, At: service.now(),
		}); err != nil {
			return err
		}
		if err := service.conversations.Touch(ctx, tx, conversation.ID, service.now()); err != nil {
			return err
		}
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil || len(active) > 0 {
			return err
		}
		return service.queueTurn(ctx, tx, conversation, bot, caller, text)
	})
}

func (service *AdminChatService) findChatBot(ctx context.Context, tx pgx.Tx, conversation runs.Conversation) (bots.Bot, error) {
	if conversation.BotID == nil {
		return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "not_an_assistant"})
	}
	bot, err := service.bots.FindByID(ctx, tx, *conversation.BotID)
	if errors.Is(err, bots.ErrBotNotFound) || (err == nil && !bot.IsEnabled()) {
		return bots.Bot{}, httpapi.NewValidationError(map[string]string{"botToken": "not_an_assistant"})
	}
	return bot, err
}

func (service *AdminChatService) queueTurn(ctx context.Context, tx pgx.Tx, conversation runs.Conversation, bot bots.Bot, caller Caller, text string) error {
	var session *string
	last, err := service.runs.FindLast(ctx, tx, conversation.ID, runs.RunKindChat)
	if err == nil {
		session = last.CLISessionID
	} else if !errors.Is(err, runs.ErrRunNotFound) {
		return err
	}
	run, err := service.runs.Create(ctx, tx, runs.NewRun{
		ConversationID: conversation.ID, BotID: bot.ID, Kind: runs.RunKindChat, RequesterUserToken: caller.UserToken, CLISessionID: session,
		Spec: runs.RunSpec{Prompt: text}, At: service.now(),
	})
	if err != nil {
		return err
	}
	return service.lifecycle.Queued(ctx, tx, run)
}

// StopChat cancels the chat's active run.
func (service *AdminChatService) StopChat(ctx context.Context, caller Caller, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		conversation, err := service.findOwnChat(ctx, tx, caller, token)
		if err != nil {
			return err
		}
		active, err := service.runs.ListActiveForConversation(ctx, tx, conversation.ID)
		if err != nil {
			return err
		}
		if len(active) == 0 {
			return runs.ErrRunNotActive
		}
		for _, run := range active {
			if err := service.lifecycle.Cancel(ctx, tx, run, runs.CauseUser, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return defaultTitle
	}
	return title
}

func isValidTitle(title string) bool {
	return len([]rune(title)) <= MaxTitleLength
}

func relayCoreError(err error) error {
	var refusal *corelink.RefusalError
	if errors.As(err, &refusal) {
		return refusal.ToAPIError()
	}
	return err
}
