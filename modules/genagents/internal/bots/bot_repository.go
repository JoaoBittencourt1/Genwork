package bots

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
)

type BotRepository struct{}

type NewBot struct {
	Token                  string
	Name                   string
	Kind                   BotKind
	AccountID              int64
	Model                  string
	Effort                 ReasoningEffort
	MaxTurns               int
	TimeoutMinutes         int
	Instructions           *string
	MemberToken            string
	MemberSecretCiphertext []byte
	At                     time.Time
}

const botSelect = `
SELECT bots.id, bots.token, bots.name, bots.kind, bots.account_id, ai_accounts.token, ai_accounts.name, ai_accounts.provider,
       bots.model, bots.effort, bots.max_turns, bots.timeout_minutes, bots.instructions, bots.member_token,
       bots.member_secret_ciphertext, bots.disabled_at, bots.created_at, bots.updated_at
FROM bots
JOIN ai_accounts ON ai_accounts.id = bots.account_id`

func (repository BotRepository) Create(ctx context.Context, db platform.DBTX, newBot NewBot) (Bot, error) {
	_, err := db.Exec(ctx, `
		INSERT INTO bots (token, name, kind, account_id, model, effort, max_turns, timeout_minutes, instructions, member_token,
		                  member_secret_ciphertext, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)`,
		newBot.Token, newBot.Name, newBot.Kind, newBot.AccountID, newBot.Model, newBot.Effort, newBot.MaxTurns, newBot.TimeoutMinutes,
		newBot.Instructions, newBot.MemberToken, newBot.MemberSecretCiphertext, newBot.At)
	if platform.IsUniqueViolation(err) {
		return Bot{}, ErrBotNameTaken
	}
	if err != nil {
		return Bot{}, fmt.Errorf("create bot: %w", err)
	}
	return repository.FindByToken(ctx, db, newBot.Token)
}

func (repository BotRepository) Update(ctx context.Context, db platform.DBTX, bot Bot, at time.Time) error {
	tag, err := db.Exec(ctx, `
		UPDATE bots SET name = $2, kind = $3, account_id = $4, model = $5, effort = $6, max_turns = $7, timeout_minutes = $8,
		                instructions = $9, updated_at = $10
		WHERE id = $1 AND deleted_at IS NULL`,
		bot.ID, bot.Name, bot.Kind, bot.AccountID, bot.Model, bot.Effort, bot.MaxTurns, bot.TimeoutMinutes, bot.Instructions, at)
	if platform.IsUniqueViolation(err) {
		return ErrBotNameTaken
	}
	if err != nil {
		return fmt.Errorf("update bot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBotNotFound
	}
	return nil
}

func (BotRepository) SetDisabledAt(ctx context.Context, db platform.DBTX, id int64, disabledAt *time.Time, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE bots SET disabled_at = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`, id, disabledAt, at)
	if err != nil {
		return fmt.Errorf("set bot status: %w", err)
	}
	return nil
}

func (BotRepository) UpdateMemberSecret(ctx context.Context, db platform.DBTX, id int64, ciphertext []byte, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE bots SET member_secret_ciphertext = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`, id, ciphertext, at)
	if err != nil {
		return fmt.Errorf("update bot secret: %w", err)
	}
	return nil
}

// SoftDelete clears the member secret in the same statement and detaches skills and servers (rules 22 and 67).
func (BotRepository) SoftDelete(ctx context.Context, db platform.DBTX, id int64, at time.Time) error {
	for _, statement := range []string{`DELETE FROM bot_skills WHERE bot_id = $1`, `DELETE FROM bot_mcp_servers WHERE bot_id = $1`} {
		if _, err := db.Exec(ctx, statement, id); err != nil {
			return fmt.Errorf("detach bot: %w", err)
		}
	}
	tag, err := db.Exec(ctx, `UPDATE bots SET deleted_at = $2, updated_at = $2, member_secret_ciphertext = NULL WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("delete bot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBotNotFound
	}
	return nil
}

func (repository BotRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Bot, error) {
	return repository.findOne(ctx, db, `bots.token = $1`, token)
}

func (repository BotRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Bot, error) {
	return repository.findOne(ctx, db, `bots.id = $1`, id)
}

func (repository BotRepository) FindByMemberToken(ctx context.Context, db platform.DBTX, memberToken string) (Bot, error) {
	return repository.findOne(ctx, db, `bots.member_token = $1`, memberToken)
}

func (repository BotRepository) findOne(ctx context.Context, db platform.DBTX, condition string, argument any) (Bot, error) {
	bot, err := scanBot(db.QueryRow(ctx, botSelect+` WHERE `+condition+` AND bots.deleted_at IS NULL`, argument))
	if errors.Is(err, pgx.ErrNoRows) {
		return Bot{}, ErrBotNotFound
	}
	if err != nil {
		return Bot{}, fmt.Errorf("find bot: %w", err)
	}
	attached, err := repository.attachReferences(ctx, db, []Bot{bot})
	if err != nil {
		return Bot{}, err
	}
	return attached[0], nil
}

// List returns the active bots, optionally of one kind, with their skills and servers in three queries (rule 21).
func (repository BotRepository) List(ctx context.Context, db platform.DBTX, kind *BotKind) ([]Bot, error) {
	query, arguments := botSelect+` WHERE bots.deleted_at IS NULL ORDER BY bots.name`, []any{}
	if kind != nil {
		query, arguments = botSelect+` WHERE bots.deleted_at IS NULL AND bots.kind = $1 ORDER BY bots.name`, []any{*kind}
	}
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list bots: %w", err)
	}
	listed, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Bot, error) { return scanBot(row) })
	if err != nil {
		return nil, fmt.Errorf("list bots: %w", err)
	}
	return repository.attachReferences(ctx, db, listed)
}

func (BotRepository) ListBotNamesUsingAccount(ctx context.Context, db platform.DBTX, accountID int64) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT name FROM bots WHERE account_id = $1 AND deleted_at IS NULL ORDER BY name`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list bots using account: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ReplaceSkills attaches exactly the skills with these tokens; an unknown token is a validation error.
func (BotRepository) ReplaceSkills(ctx context.Context, db platform.DBTX, botID int64, skillTokens []string) error {
	return replaceJoin(ctx, db, botID, skillTokens, "bot_skills", "skill_id", "skills", "skillTokens")
}

func (BotRepository) ReplaceMCPServers(ctx context.Context, db platform.DBTX, botID int64, serverTokens []string) error {
	return replaceJoin(ctx, db, botID, serverTokens, "bot_mcp_servers", "mcp_server_id", "mcp_servers", "mcpServerTokens")
}

func replaceJoin(ctx context.Context, db platform.DBTX, botID int64, tokens []string, joinTable, joinColumn, targetTable, field string) error {
	if _, err := db.Exec(ctx, `DELETE FROM `+joinTable+` WHERE bot_id = $1`, botID); err != nil {
		return fmt.Errorf("clear %s: %w", joinTable, err)
	}
	if len(tokens) == 0 {
		return nil
	}
	tag, err := db.Exec(ctx, `
		INSERT INTO `+joinTable+` (bot_id, `+joinColumn+`)
		SELECT $1, id FROM `+targetTable+` WHERE token = ANY($2) AND deleted_at IS NULL`, botID, tokens)
	if err != nil {
		return fmt.Errorf("fill %s: %w", joinTable, err)
	}
	if int(tag.RowsAffected()) != len(tokens) {
		return httpapi.NewValidationError(map[string]string{field: "unknown"})
	}
	return nil
}

func (BotRepository) ListSkillIDs(ctx context.Context, db platform.DBTX, botID int64) ([]int64, error) {
	rows, err := db.Query(ctx, `SELECT skill_id FROM bot_skills WHERE bot_id = $1`, botID)
	if err != nil {
		return nil, fmt.Errorf("list bot skills: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (BotRepository) ListMCPServerIDs(ctx context.Context, db platform.DBTX, botID int64) ([]int64, error) {
	rows, err := db.Query(ctx, `SELECT mcp_server_id FROM bot_mcp_servers WHERE bot_id = $1`, botID)
	if err != nil {
		return nil, fmt.Errorf("list bot mcp servers: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (BotRepository) attachReferences(ctx context.Context, db platform.DBTX, listed []Bot) ([]Bot, error) {
	ids := make([]int64, 0, len(listed))
	for _, bot := range listed {
		ids = append(ids, bot.ID)
	}
	skills, err := listReferences(ctx, db, ids, `SELECT bot_skills.bot_id, skills.token, skills.name FROM bot_skills
		JOIN skills ON skills.id = bot_skills.skill_id WHERE bot_skills.bot_id = ANY($1) AND skills.deleted_at IS NULL ORDER BY skills.name`)
	if err != nil {
		return nil, err
	}
	servers, err := listReferences(ctx, db, ids, `SELECT bot_mcp_servers.bot_id, mcp_servers.token, mcp_servers.name FROM bot_mcp_servers
		JOIN mcp_servers ON mcp_servers.id = bot_mcp_servers.mcp_server_id WHERE bot_mcp_servers.bot_id = ANY($1) AND mcp_servers.deleted_at IS NULL ORDER BY mcp_servers.name`)
	if err != nil {
		return nil, err
	}
	for index := range listed {
		listed[index].Skills, listed[index].MCPServers = skills[listed[index].ID], servers[listed[index].ID]
	}
	return listed, nil
}

func listReferences(ctx context.Context, db platform.DBTX, botIDs []int64, query string) (map[int64][]Reference, error) {
	rows, err := db.Query(ctx, query, botIDs)
	if err != nil {
		return nil, fmt.Errorf("list bot references: %w", err)
	}
	defer rows.Close()
	byBot := map[int64][]Reference{}
	for rows.Next() {
		var botID int64
		var reference Reference
		if err := rows.Scan(&botID, &reference.Token, &reference.Name); err != nil {
			return nil, fmt.Errorf("scan bot reference: %w", err)
		}
		byBot[botID] = append(byBot[botID], reference)
	}
	return byBot, rows.Err()
}

func scanBot(row pgx.Row) (Bot, error) {
	var bot Bot
	err := row.Scan(&bot.ID, &bot.Token, &bot.Name, &bot.Kind, &bot.AccountID, &bot.AccountToken, &bot.AccountName, &bot.Provider,
		&bot.Model, &bot.Effort, &bot.MaxTurns, &bot.TimeoutMinutes, &bot.Instructions, &bot.MemberToken,
		&bot.MemberSecretCiphertext, &bot.DisabledAt, &bot.CreatedAt, &bot.UpdatedAt)
	return bot, err
}
