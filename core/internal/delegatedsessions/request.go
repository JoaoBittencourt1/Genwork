package delegatedsessions

import (
	"strings"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const maxPurposeLength = 100

type GrantRequest struct {
	Module  string `json:"module"`
	Purpose string `json:"purpose"`
}

type GrantInput struct {
	Module  string
	Purpose string
}

func (request GrantRequest) ToGrantInput() (GrantInput, error) {
	input := GrantInput{Module: strings.TrimSpace(request.Module), Purpose: strings.TrimSpace(request.Purpose)}
	fields := map[string]string{}
	if input.Module == "" {
		fields["module"] = "required"
	}
	if length := utf8.RuneCountInString(input.Purpose); length == 0 || length > maxPurposeLength {
		fields["purpose"] = "invalid"
	}
	if len(fields) > 0 {
		return GrantInput{}, platform.NewValidationError(fields)
	}
	return input, nil
}
