package accounts

import (
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const maxAPIKeyLength = 512

type CreateAiAccountRequest struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Mode        string `json:"mode"`
	APIKey      string `json:"apiKey"`
	RunnerToken string `json:"runnerToken"`
}

type CreateAiAccountInput struct {
	Name        string
	Provider    Provider
	Mode        AccountMode
	APIKey      string
	RunnerToken string
}

func (request CreateAiAccountRequest) Validate() (CreateAiAccountInput, error) {
	fields := map[string]string{}
	input := CreateAiAccountInput{
		Name: NormalizeAccountName(request.Name), Provider: Provider(request.Provider), Mode: AccountMode(request.Mode),
		APIKey: request.APIKey, RunnerToken: request.RunnerToken,
	}
	if !IsValidAccountName(input.Name) {
		fields["name"] = "invalid"
	}
	definition, knownProvider := FindProviderDefinition(input.Provider)
	if !knownProvider {
		fields["provider"] = "invalid"
	}
	switch {
	case input.Mode != AccountModeSubscription && input.Mode != AccountModeAPIKey:
		fields["mode"] = "invalid"
	case knownProvider && !definition.Supports(input.Mode):
		fields["mode"] = "unsupported_by_provider"
	case input.Mode == AccountModeAPIKey && !isValidAPIKey(input.APIKey):
		fields["apiKey"] = "required"
	case input.Mode == AccountModeSubscription && input.RunnerToken == "":
		fields["runnerToken"] = "required"
	}
	if len(fields) > 0 {
		return CreateAiAccountInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}

func isValidAPIKey(apiKey string) bool {
	length := utf8.RuneCountInString(apiKey)
	return length > 0 && length <= maxAPIKeyLength
}

// UpdateAiAccountRequest is a PATCH body: omitted fields stay unchanged and neither accepts null (rule 17).
type UpdateAiAccountRequest struct {
	Name   httpapi.PatchField[string] `json:"name"`
	APIKey httpapi.PatchField[string] `json:"apiKey"`
}

type AiAccountChanges struct {
	Name   *string
	APIKey *string
}

func (request UpdateAiAccountRequest) Validate() (AiAccountChanges, error) {
	fields := map[string]string{}
	var changes AiAccountChanges
	if request.Name.Null {
		fields["name"] = "not_nullable"
	} else if request.Name.Present {
		name := NormalizeAccountName(request.Name.Value)
		if !IsValidAccountName(name) {
			fields["name"] = "invalid"
		}
		changes.Name = &name
	}
	if request.APIKey.Null {
		fields["apiKey"] = "not_nullable"
	} else if request.APIKey.Present {
		if !isValidAPIKey(request.APIKey.Value) {
			fields["apiKey"] = "invalid"
		}
		apiKey := request.APIKey.Value
		changes.APIKey = &apiKey
	}
	if len(fields) > 0 {
		return AiAccountChanges{}, httpapi.NewValidationError(fields)
	}
	return changes, nil
}

type AddAccountModelRequest struct {
	ModelID     string `json:"modelId"`
	DisplayName string `json:"displayName"`
}

type AddAccountModelInput struct {
	ModelID     string
	DisplayName string
}

func (request AddAccountModelRequest) Validate() (AddAccountModelInput, error) {
	fields := map[string]string{}
	input := AddAccountModelInput{ModelID: trimSpace(request.ModelID), DisplayName: trimSpace(request.DisplayName)}
	if length := utf8.RuneCountInString(input.ModelID); length == 0 || length > MaxModelIDLength {
		fields["modelId"] = "invalid"
	}
	if input.DisplayName == "" {
		input.DisplayName = input.ModelID
	}
	if utf8.RuneCountInString(input.DisplayName) > MaxModelDisplayLength {
		fields["displayName"] = "invalid"
	}
	if len(fields) > 0 {
		return AddAccountModelInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}
