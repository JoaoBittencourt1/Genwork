package accounts

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxAccountNameLength  = 80
	MaxModelIDLength      = 120
	MaxModelDisplayLength = 120
	apiKeyLast4Length     = 4
)

var (
	ErrAccountNotFound     = httpapi.NewAPIError(http.StatusNotFound, "account_not_found")
	ErrAccountNameTaken    = httpapi.NewAPIError(http.StatusConflict, "account_name_taken")
	ErrModelNotFound       = httpapi.NewAPIError(http.StatusNotFound, "model_not_found")
	ErrModelAlreadyDefined = httpapi.NewAPIError(http.StatusConflict, "model_already_defined")
)

// AiAccount is an AI provider account. APIKeyCiphertext is set only for API-key accounts and is cleared when the
// account is removed (rule 67); RunnerID is set only for subscription accounts.
type AiAccount struct {
	ID               int64
	Token            string
	Name             string
	Provider         Provider
	Mode             AccountMode
	RunnerID         *int64
	RunnerToken      *string
	APIKeyCiphertext []byte
	APIKeyLast4      *string
	Status           AccountStatus
	LastError        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (account AiAccount) HasAPIKey() bool {
	return account.APIKeyLast4 != nil
}

func NormalizeAccountName(name string) string {
	return strings.TrimSpace(name)
}

func IsValidAccountName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxAccountNameLength
}

// APIKeyLast4 keeps only what the screens may show of a key.
func LastCharactersOfKey(apiKey string) string {
	runes := []rune(apiKey)
	if len(runes) <= apiKeyLast4Length {
		return string(runes)
	}
	return string(runes[len(runes)-apiKeyLast4Length:])
}

// AccountModel is a custom model id an admin added to one account, for models released after the provider catalog.
type AccountModel struct {
	ID          int64
	Token       string
	AccountID   int64
	ModelID     string
	DisplayName string
	CreatedAt   time.Time
}

func trimSpace(value string) string {
	return strings.TrimSpace(value)
}
