package bots

import (
	"strings"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type CreateBotRequest struct {
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	AccountToken    string   `json:"accountToken"`
	Model           string   `json:"model"`
	Role            string   `json:"role"`
	Instructions    *string  `json:"instructions"`
	Effort          string   `json:"effort"`
	MaxTurns        *int     `json:"maxTurns"`
	TimeoutMinutes  *int     `json:"timeoutMinutes"`
	SkillTokens     []string `json:"skillTokens"`
	MCPServerTokens []string `json:"mcpServerTokens"`
}

type CreateBotInput struct {
	Name            string
	Kind            BotKind
	AccountToken    string
	Model           string
	RoleToken       string
	Instructions    *string
	Effort          ReasoningEffort
	MaxTurns        int
	TimeoutMinutes  int
	SkillTokens     []string
	MCPServerTokens []string
}

func (request CreateBotRequest) Validate() (CreateBotInput, error) {
	fields := map[string]string{}
	input := CreateBotInput{
		Name: NormalizeName(request.Name), Kind: BotKind(request.Kind), AccountToken: request.AccountToken, Model: strings.TrimSpace(request.Model),
		RoleToken: request.Role, Instructions: request.Instructions, Effort: ReasoningEffort(request.Effort),
		MaxTurns: DefaultMaxTurns, TimeoutMinutes: DefaultTimeoutMinutes, SkillTokens: request.SkillTokens, MCPServerTokens: request.MCPServerTokens,
	}
	if input.Effort == "" {
		input.Effort = EffortDefault
	}
	if request.MaxTurns != nil {
		input.MaxTurns = *request.MaxTurns
	}
	if request.TimeoutMinutes != nil {
		input.TimeoutMinutes = *request.TimeoutMinutes
	}
	if !IsValidName(input.Name) {
		fields["name"] = "invalid"
	}
	if !input.Kind.IsValid() {
		fields["kind"] = "invalid"
	}
	if input.AccountToken == "" {
		fields["accountToken"] = "required"
	}
	if input.Model == "" {
		fields["model"] = "required"
	}
	if input.RoleToken == "" {
		fields["role"] = "required"
	}
	mergeParameterFields(fields, input.Effort, input.MaxTurns, input.TimeoutMinutes, input.Instructions)
	if len(fields) > 0 {
		return CreateBotInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}

func mergeParameterFields(fields map[string]string, effort ReasoningEffort, maxTurns, timeoutMinutes int, instructions *string) {
	if !effort.IsValid() {
		fields["effort"] = "invalid"
	}
	if maxTurns < MinMaxTurns || maxTurns > MaxMaxTurns {
		fields["maxTurns"] = "invalid"
	}
	if timeoutMinutes < MinTimeoutMinutes || timeoutMinutes > MaxTimeoutMinutes {
		fields["timeoutMinutes"] = "invalid"
	}
	if instructions != nil && utf8.RuneCountInString(*instructions) > MaxInstructionsLength {
		fields["instructions"] = "too_long"
	}
}

// UpdateBotRequest is a PATCH body (rule 17): omitted fields stay unchanged, null clears instructions and is rejected
// for every required field.
type UpdateBotRequest struct {
	Name            httpapi.PatchField[string]   `json:"name"`
	Kind            httpapi.PatchField[string]   `json:"kind"`
	AccountToken    httpapi.PatchField[string]   `json:"accountToken"`
	Model           httpapi.PatchField[string]   `json:"model"`
	Role            httpapi.PatchField[string]   `json:"role"`
	Instructions    httpapi.PatchField[string]   `json:"instructions"`
	Effort          httpapi.PatchField[string]   `json:"effort"`
	MaxTurns        httpapi.PatchField[int]      `json:"maxTurns"`
	TimeoutMinutes  httpapi.PatchField[int]      `json:"timeoutMinutes"`
	SkillTokens     httpapi.PatchField[[]string] `json:"skillTokens"`
	MCPServerTokens httpapi.PatchField[[]string] `json:"mcpServerTokens"`
	Enabled         httpapi.PatchField[bool]     `json:"enabled"`
}

type BotChanges struct {
	Name               *string
	Kind               *BotKind
	AccountToken       *string
	Model              *string
	RoleToken          *string
	Instructions       *string
	ClearsInstructions bool
	Effort             *ReasoningEffort
	MaxTurns           *int
	TimeoutMinutes     *int
	SkillTokens        *[]string
	MCPServerTokens    *[]string
	Enabled            *bool
}

func (request UpdateBotRequest) Validate() (BotChanges, error) {
	fields := map[string]string{}
	var changes BotChanges
	required := map[string]bool{
		"name": request.Name.Null, "kind": request.Kind.Null, "accountToken": request.AccountToken.Null, "model": request.Model.Null,
		"role": request.Role.Null, "effort": request.Effort.Null, "maxTurns": request.MaxTurns.Null,
		"timeoutMinutes": request.TimeoutMinutes.Null, "skillTokens": request.SkillTokens.Null,
		"mcpServerTokens": request.MCPServerTokens.Null, "enabled": request.Enabled.Null,
	}
	for field, null := range required {
		if null {
			fields[field] = "not_nullable"
		}
	}
	if request.Name.HasValue() {
		name := NormalizeName(request.Name.Value)
		changes.Name = &name
		if !IsValidName(name) {
			fields["name"] = "invalid"
		}
	}
	if request.Kind.HasValue() {
		kind := BotKind(request.Kind.Value)
		changes.Kind = &kind
		if !kind.IsValid() {
			fields["kind"] = "invalid"
		}
	}
	if request.AccountToken.HasValue() {
		changes.AccountToken = &request.AccountToken.Value
	}
	if request.Model.HasValue() {
		model := strings.TrimSpace(request.Model.Value)
		changes.Model = &model
		if model == "" {
			fields["model"] = "required"
		}
	}
	if request.Role.HasValue() {
		changes.RoleToken = &request.Role.Value
	}
	if request.Instructions.Present {
		changes.ClearsInstructions = request.Instructions.Null
		if request.Instructions.HasValue() {
			changes.Instructions = &request.Instructions.Value
		}
	}
	if request.Effort.HasValue() {
		effort := ReasoningEffort(request.Effort.Value)
		changes.Effort = &effort
	}
	if request.MaxTurns.HasValue() {
		changes.MaxTurns = &request.MaxTurns.Value
	}
	if request.TimeoutMinutes.HasValue() {
		changes.TimeoutMinutes = &request.TimeoutMinutes.Value
	}
	if request.SkillTokens.HasValue() {
		changes.SkillTokens = &request.SkillTokens.Value
	}
	if request.MCPServerTokens.HasValue() {
		changes.MCPServerTokens = &request.MCPServerTokens.Value
	}
	if request.Enabled.HasValue() {
		changes.Enabled = &request.Enabled.Value
	}
	validateChangedParameters(fields, changes)
	if len(fields) > 0 {
		return BotChanges{}, httpapi.NewValidationError(fields)
	}
	return changes, nil
}

func validateChangedParameters(fields map[string]string, changes BotChanges) {
	effort, maxTurns, timeout := EffortDefault, MinMaxTurns, MinTimeoutMinutes
	if changes.Effort != nil {
		effort = *changes.Effort
	}
	if changes.MaxTurns != nil {
		maxTurns = *changes.MaxTurns
	}
	if changes.TimeoutMinutes != nil {
		timeout = *changes.TimeoutMinutes
	}
	checked := map[string]string{}
	mergeParameterFields(checked, effort, maxTurns, timeout, changes.Instructions)
	for field, problem := range checked {
		present := map[string]bool{
			"effort": changes.Effort != nil, "maxTurns": changes.MaxTurns != nil,
			"timeoutMinutes": changes.TimeoutMinutes != nil, "instructions": changes.Instructions != nil,
		}
		if present[field] {
			fields[field] = problem
		}
	}
}
