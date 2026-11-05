package members

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

type CreateBotRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (request CreateBotRequest) Validate() error {
	fields := map[string]string{}
	if !users.IsValidName(users.NormalizeName(request.Name)) {
		fields["name"] = "invalid"
	}
	if request.Role == "" {
		fields["role"] = "required"
	}
	if len(fields) > 0 {
		return platform.NewValidationError(fields)
	}
	return nil
}

// UpdateBotRequest is a PATCH body: omitted fields stay unchanged and none of them accepts null (rule 17).
type UpdateBotRequest struct {
	Name   platform.PatchField[string] `json:"name"`
	Role   platform.PatchField[string] `json:"role"`
	Status platform.PatchField[string] `json:"status"`
}

type BotUpdateInput struct {
	Name      *string
	RoleToken *string
	Status    *MemberStatus
}

func (request UpdateBotRequest) ToUpdateInput() (BotUpdateInput, error) {
	fields := map[string]string{}
	var input BotUpdateInput
	if request.Name.Null {
		fields["name"] = "not_nullable"
	} else if request.Name.Present {
		name := users.NormalizeName(request.Name.Value)
		if !users.IsValidName(name) {
			fields["name"] = "invalid"
		}
		input.Name = &name
	}
	if request.Role.Null {
		fields["role"] = "not_nullable"
	} else if request.Role.Present {
		input.RoleToken = &request.Role.Value
	}
	if request.Status.Null {
		fields["status"] = "not_nullable"
	} else if request.Status.Present {
		status, valid := ParseMemberStatus(request.Status.Value)
		if !valid {
			fields["status"] = "invalid"
		}
		input.Status = &status
	}
	if len(fields) > 0 {
		return BotUpdateInput{}, platform.NewValidationError(fields)
	}
	return input, nil
}
