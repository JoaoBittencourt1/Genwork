package accounts

import "slices"

type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderCursor    Provider = "cursor"
	ProviderXAI       Provider = "xai"
)

type AccountMode string

const (
	AccountModeSubscription AccountMode = "subscription"
	AccountModeAPIKey       AccountMode = "api_key"
)

type AccountStatus string

const (
	AccountStatusPendingSignIn AccountStatus = "pending_sign_in"
	AccountStatusReady         AccountStatus = "ready"
	AccountStatusError         AccountStatus = "error"
)

type CatalogModel struct {
	ID          string
	DisplayName string
}

// ProviderDefinition is the single owner of what a provider offers (rule 7): its CLI, the account modes it supports,
// and its model catalog.
type ProviderDefinition struct {
	Provider Provider
	// CLI is the tool name a runner reports in its heartbeat for this provider.
	CLI    string
	Modes  []AccountMode
	Models []CatalogModel
}

var providerDefinitions = []ProviderDefinition{
	{
		Provider: ProviderAnthropic, CLI: "claude", Modes: []AccountMode{AccountModeSubscription, AccountModeAPIKey},
		Models: []CatalogModel{
			{ID: "claude-fable-5-1", DisplayName: "Claude Fable 5.1"},
			{ID: "claude-opus-5-5", DisplayName: "Claude Opus 5.5"},
			{ID: "claude-sonnet-5-5", DisplayName: "Claude Sonnet 5.5"},
			{ID: "claude-haiku-5-5", DisplayName: "Claude Haiku 5.5"},
		},
	},
	{
		Provider: ProviderOpenAI, CLI: "codex", Modes: []AccountMode{AccountModeSubscription, AccountModeAPIKey},
		Models: []CatalogModel{{ID: "gpt-5", DisplayName: "GPT-5"}, {ID: "gpt-5-codex", DisplayName: "GPT-5 Codex"}},
	},
	{
		Provider: ProviderCursor, CLI: "cursor-agent", Modes: []AccountMode{AccountModeSubscription, AccountModeAPIKey},
		Models: []CatalogModel{{ID: "auto", DisplayName: "Auto"}},
	},
	{
		Provider: ProviderXAI, CLI: "grok", Modes: []AccountMode{AccountModeAPIKey},
		Models: []CatalogModel{{ID: "grok-4", DisplayName: "Grok 4"}, {ID: "grok-code-fast-1", DisplayName: "Grok Code Fast"}},
	},
}

func ProviderDefinitions() []ProviderDefinition {
	return slices.Clone(providerDefinitions)
}

func FindProviderDefinition(provider Provider) (ProviderDefinition, bool) {
	for _, definition := range providerDefinitions {
		if definition.Provider == provider {
			return definition, true
		}
	}
	return ProviderDefinition{}, false
}

func (definition ProviderDefinition) Supports(mode AccountMode) bool {
	return slices.Contains(definition.Modes, mode)
}

func (definition ProviderDefinition) HasModel(modelID string) bool {
	for _, model := range definition.Models {
		if model.ID == modelID {
			return true
		}
	}
	return false
}
