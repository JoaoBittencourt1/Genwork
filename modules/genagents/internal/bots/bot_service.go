package bots

import (
	"context"
	"errors"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const memberSecretColumn = "bots.member_secret"

const cancelReasonBotDisabled = "bot_disabled"

// BotMemberClient is what bots need from core (rule 9): the bot's member, created and kept in step with the bot.
type BotMemberClient interface {
	CreateBotMember(ctx context.Context, accessToken, name, roleToken string) (corelink.BotMember, error)
	RenameBotMember(ctx context.Context, accessToken, botToken, name string) error
	ChangeBotMemberRole(ctx context.Context, accessToken, botToken, roleToken string) error
	SetBotMemberActive(ctx context.Context, accessToken, botToken string, active bool) error
	RemoveBotMember(ctx context.Context, accessToken, botToken string) error
	RotateBotMemberSecret(ctx context.Context, accessToken, botToken string) (string, error)
}

// RunCanceller ends a bot's active runs when the bot is disabled or removed.
type RunCanceller interface {
	CancelActiveRunsForBot(ctx context.Context, db platform.DBTX, botID int64, reason string) error
}

type ModelChecker interface {
	ModelIsAvailable(ctx context.Context, db platform.DBTX, account accounts.AiAccount, modelID string) (bool, error)
}

type Caller struct {
	AccessToken string
}

type BotService struct {
	db       *pgxpool.Pool
	cipher   *platform.SecretCipher
	members  BotMemberClient
	canceler RunCanceller
	models   ModelChecker
	accounts accounts.AiAccountRepository
	bots     BotRepository
	now      func() time.Time
}

func NewBotService(db *pgxpool.Pool, cipher *platform.SecretCipher, members BotMemberClient, canceler RunCanceller, models ModelChecker, now func() time.Time) *BotService {
	return &BotService{db: db, cipher: cipher, members: members, canceler: canceler, models: models, now: now}
}

func MemberSecretAssociatedData(botToken string) string {
	return memberSecretColumn + "|" + botToken
}

func (service *BotService) ListBots(ctx context.Context, kind *BotKind) ([]Bot, error) {
	return service.bots.List(ctx, service.db, kind)
}

func (service *BotService) FindBot(ctx context.Context, token string) (Bot, error) {
	return service.bots.FindByToken(ctx, service.db, token)
}

// CreateBot creates the core bot member first, then the bot; when the bot cannot be stored, the member is removed again,
// so a refusal on either side leaves no partial state.
func (service *BotService) CreateBot(ctx context.Context, caller Caller, input CreateBotInput) (Bot, error) {
	account, err := service.findAccountWithModel(ctx, service.db, input.AccountToken, input.Model)
	if err != nil {
		return Bot{}, err
	}
	member, err := service.members.CreateBotMember(ctx, caller.AccessToken, input.Name, input.RoleToken)
	if err != nil {
		return Bot{}, mapCoreError(err)
	}
	botToken := platform.NewPublicToken()
	ciphertext, err := service.cipher.Encrypt(member.ClientSecret, MemberSecretAssociatedData(botToken))
	if err != nil {
		service.removeMemberQuietly(ctx, caller, member.Token)
		return Bot{}, err
	}
	var created Bot
	err = pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		created, err = service.bots.Create(ctx, tx, NewBot{
			Token: botToken, Name: input.Name, Kind: input.Kind, AccountID: account.ID, Model: input.Model, Effort: input.Effort,
			MaxTurns: input.MaxTurns, TimeoutMinutes: input.TimeoutMinutes, Instructions: input.Instructions,
			MemberToken: member.Token, MemberSecretCiphertext: ciphertext, At: service.now(),
		})
		if err != nil {
			return err
		}
		if err := service.bots.ReplaceSkills(ctx, tx, created.ID, input.SkillTokens); err != nil {
			return err
		}
		return service.bots.ReplaceMCPServers(ctx, tx, created.ID, input.MCPServerTokens)
	})
	if err != nil {
		service.removeMemberQuietly(ctx, caller, member.Token)
		return Bot{}, err
	}
	return service.bots.FindByToken(ctx, service.db, created.Token)
}

func (service *BotService) removeMemberQuietly(ctx context.Context, caller Caller, memberToken string) {
	_ = service.members.RemoveBotMember(ctx, caller.AccessToken, memberToken)
}

// UpdateBot changes the bot and its member in one transaction: core is called before commit, so a core refusal rolls
// the bot change back.
func (service *BotService) UpdateBot(ctx context.Context, caller Caller, token string, changes BotChanges) (Bot, error) {
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		bot, err := service.bots.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		updated, err := service.applyFieldChanges(ctx, tx, bot, changes)
		if err != nil {
			return err
		}
		if err := service.bots.Update(ctx, tx, updated, service.now()); err != nil {
			return err
		}
		if err := service.replaceAttachments(ctx, tx, bot, changes); err != nil {
			return err
		}
		return service.syncMember(ctx, tx, caller, bot, changes)
	})
	if err != nil {
		return Bot{}, err
	}
	return service.bots.FindByToken(ctx, service.db, token)
}

func (service *BotService) applyFieldChanges(ctx context.Context, tx pgx.Tx, bot Bot, changes BotChanges) (Bot, error) {
	if changes.Name != nil {
		bot.Name = *changes.Name
	}
	if changes.Kind != nil {
		bot.Kind = *changes.Kind
	}
	if changes.Effort != nil {
		bot.Effort = *changes.Effort
	}
	if changes.MaxTurns != nil {
		bot.MaxTurns = *changes.MaxTurns
	}
	if changes.TimeoutMinutes != nil {
		bot.TimeoutMinutes = *changes.TimeoutMinutes
	}
	if changes.ClearsInstructions {
		bot.Instructions = nil
	} else if changes.Instructions != nil {
		bot.Instructions = changes.Instructions
	}
	if changes.AccountToken != nil || changes.Model != nil {
		accountToken, model := bot.AccountToken, bot.Model
		if changes.AccountToken != nil {
			accountToken = *changes.AccountToken
		}
		if changes.Model != nil {
			model = *changes.Model
		}
		account, err := service.findAccountWithModel(ctx, tx, accountToken, model)
		if err != nil {
			return Bot{}, err
		}
		bot.AccountID, bot.Model = account.ID, model
	}
	return bot, nil
}

func (service *BotService) replaceAttachments(ctx context.Context, tx pgx.Tx, bot Bot, changes BotChanges) error {
	if changes.SkillTokens != nil {
		if err := service.bots.ReplaceSkills(ctx, tx, bot.ID, *changes.SkillTokens); err != nil {
			return err
		}
	}
	if changes.MCPServerTokens != nil {
		return service.bots.ReplaceMCPServers(ctx, tx, bot.ID, *changes.MCPServerTokens)
	}
	return nil
}

// syncMember mirrors name, role, and status on the bot's core member; disabling also cancels the bot's active runs.
func (service *BotService) syncMember(ctx context.Context, tx pgx.Tx, caller Caller, bot Bot, changes BotChanges) error {
	if changes.Name != nil && *changes.Name != bot.Name {
		if err := service.members.RenameBotMember(ctx, caller.AccessToken, bot.MemberToken, *changes.Name); err != nil {
			return mapCoreError(err)
		}
	}
	if changes.RoleToken != nil {
		if err := service.members.ChangeBotMemberRole(ctx, caller.AccessToken, bot.MemberToken, *changes.RoleToken); err != nil {
			return mapCoreError(err)
		}
	}
	if changes.Enabled == nil || *changes.Enabled == bot.IsEnabled() {
		return nil
	}
	if err := service.members.SetBotMemberActive(ctx, caller.AccessToken, bot.MemberToken, *changes.Enabled); err != nil {
		return mapCoreError(err)
	}
	var disabledAt *time.Time
	if !*changes.Enabled {
		now := service.now()
		disabledAt = &now
		if err := service.canceler.CancelActiveRunsForBot(ctx, tx, bot.ID, cancelReasonBotDisabled); err != nil {
			return err
		}
	}
	return service.bots.SetDisabledAt(ctx, tx, bot.ID, disabledAt, service.now())
}

func (service *BotService) DeleteBot(ctx context.Context, caller Caller, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		bot, err := service.bots.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		if err := service.canceler.CancelActiveRunsForBot(ctx, tx, bot.ID, cancelReasonBotDisabled); err != nil {
			return err
		}
		if err := service.members.RemoveBotMember(ctx, caller.AccessToken, bot.MemberToken); err != nil {
			return mapCoreError(err)
		}
		return service.bots.SoftDelete(ctx, tx, bot.ID, service.now())
	})
}

// RotateSecret rotates the member's secret in core and stores the new one, so the bot keeps signing in.
func (service *BotService) RotateSecret(ctx context.Context, caller Caller, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		bot, err := service.bots.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		secret, err := service.members.RotateBotMemberSecret(ctx, caller.AccessToken, bot.MemberToken)
		if err != nil {
			return mapCoreError(err)
		}
		ciphertext, err := service.cipher.Encrypt(secret, MemberSecretAssociatedData(bot.Token))
		if err != nil {
			return err
		}
		return service.bots.UpdateMemberSecret(ctx, tx, bot.ID, ciphertext, service.now())
	})
}

// DecryptMemberSecret is how a run signs its bot in; it never leaves genagents.
func (service *BotService) DecryptMemberSecret(bot Bot) (string, error) {
	return service.cipher.Decrypt(bot.MemberSecretCiphertext, MemberSecretAssociatedData(bot.Token))
}

func (service *BotService) findAccountWithModel(ctx context.Context, db platform.DBTX, accountToken, model string) (accounts.AiAccount, error) {
	account, err := service.accounts.FindByToken(ctx, db, accountToken)
	if errors.Is(err, accounts.ErrAccountNotFound) {
		return accounts.AiAccount{}, httpapi.NewValidationError(map[string]string{"accountToken": "unknown"})
	}
	if err != nil {
		return accounts.AiAccount{}, err
	}
	available, err := service.models.ModelIsAvailable(ctx, db, account, model)
	if err != nil {
		return accounts.AiAccount{}, err
	}
	if !available {
		return accounts.AiAccount{}, httpapi.NewValidationError(map[string]string{"model": "not_available_for_account"})
	}
	return account, nil
}

// mapCoreError relays a core refusal with its own status and code, so a missing permission stays a 403.
func mapCoreError(err error) error {
	var refusal *corelink.RefusalError
	if errors.As(err, &refusal) {
		return refusal.ToAPIError()
	}
	return err
}
