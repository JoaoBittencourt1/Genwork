package members

import "github.com/JoaoBittencourt1/Genwork/core/internal/platform"

// UpdateMemberRequest is a PATCH body: omitted fields stay unchanged and none of them accepts null (rule 17).
type UpdateMemberRequest struct {
	Role              platform.PatchField[string]   `json:"role"`
	Status            platform.PatchField[string]   `json:"status"`
	RestrictedModules platform.PatchField[[]string] `json:"restrictedModules"`
}

type UpdateInput struct {
	RoleToken         *string
	Status            *MemberStatus
	RestrictedModules *[]string
}

func (request UpdateMemberRequest) ToUpdateInput() (UpdateInput, error) {
	fields := map[string]string{}
	var input UpdateInput
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
	if request.RestrictedModules.Null {
		fields["restrictedModules"] = "not_nullable"
	} else if request.RestrictedModules.Present {
		restrictedModules := append([]string{}, request.RestrictedModules.Value...)
		input.RestrictedModules = &restrictedModules
	}
	if len(fields) > 0 {
		return UpdateInput{}, platform.NewValidationError(fields)
	}
	return input, nil
}
