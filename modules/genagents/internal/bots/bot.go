package bots

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxNameLength         = 80
	MaxInstructionsLength = 20000
	MinMaxTurns           = 1
	MaxMaxTurns           = 500
	MinTimeoutMinutes     = 5
	MaxTimeoutMinutes     = 720
	DefaultMaxTurns       = 100
	DefaultTimeoutMinutes = 60
)

type BotKind string

const (
	BotKindDev       BotKind = "dev"
	BotKindReviewer  BotKind = "reviewer"
	BotKindAssistant BotKind = "assistant"
)

func (kind BotKind) IsValid() bool {
	return kind == BotKindDev || kind == BotKindReviewer || kind == BotKindAssistant
}

type ReasoningEffort string

const (
	EffortDefault ReasoningEffort = "default"
	EffortLow     ReasoningEffort = "low"
	EffortMedium  ReasoningEffort = "medium"
	EffortHigh    ReasoningEffort = "high"
)

func (effort ReasoningEffort) IsValid() bool {
	switch effort {
	case EffortDefault, EffortLow, EffortMedium, EffortHigh:
		return true
	}
	return false
}

var (
	ErrBotNotFound  = httpapi.NewAPIError(http.StatusNotFound, "bot_not_found")
	ErrBotNameTaken = httpapi.NewAPIError(http.StatusConflict, "bot_name_taken")
)

type Reference struct {
	Token string
	Name  string
}

// Bot is a configured AI worker backed by a core bot member. MemberSecretCiphertext is the member's client secret,
// which genagents must use again to sign the bot in, so it is stored encrypted and never returned (rule 67).
type Bot struct {
	ID                     int64
	Token                  string
	Name                   string
	Kind                   BotKind
	AccountID              int64
	AccountToken           string
	AccountName            string
	Provider               accounts.Provider
	Model                  string
	Effort                 ReasoningEffort
	MaxTurns               int
	TimeoutMinutes         int
	Instructions           *string
	MemberToken            string
	MemberSecretCiphertext []byte
	DisabledAt             *time.Time
	Skills                 []Reference
	MCPServers             []Reference
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (bot Bot) IsEnabled() bool {
	return bot.DisabledAt == nil
}

func (bot Bot) Timeout() time.Duration {
	return time.Duration(bot.TimeoutMinutes) * time.Minute
}

func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

func IsValidName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxNameLength
}
