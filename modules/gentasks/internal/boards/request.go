package boards

import "github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"

type CreateBoardRequest struct {
	Name string `json:"name"`
}

func (request CreateBoardRequest) NormalizedName() (string, error) {
	name := NormalizeName(request.Name)
	if !IsValidBoardName(name) {
		return "", httpapi.NewValidationError(map[string]string{"name": "invalid"})
	}
	return name, nil
}

type UpdateBoardRequest struct {
	Name httpapi.PatchField[string] `json:"name"`
}

// NormalizedName returns nil when the name was omitted; the board name is not nullable (rule 17).
func (request UpdateBoardRequest) NormalizedName() (*string, error) {
	if !request.Name.Present {
		return nil, nil
	}
	name := NormalizeName(request.Name.Value)
	if request.Name.Null || !IsValidBoardName(name) {
		return nil, httpapi.NewValidationError(map[string]string{"name": "invalid"})
	}
	return &name, nil
}

type CreateColumnRequest struct {
	Name string `json:"name"`
}

func (request CreateColumnRequest) NormalizedName() (string, error) {
	name := NormalizeName(request.Name)
	if !IsValidColumnName(name) {
		return "", httpapi.NewValidationError(map[string]string{"name": "invalid"})
	}
	return name, nil
}

type UpdateColumnRequest struct {
	Name httpapi.PatchField[string] `json:"name"`
}

// ColumnNameChange describes a PATCH of a column name: nothing, a new name, or clearing it.
type ColumnNameChange struct {
	Requested bool
	Name      *string
}

func (request UpdateColumnRequest) NameChange() (ColumnNameChange, error) {
	if !request.Name.Present {
		return ColumnNameChange{}, nil
	}
	if request.Name.Null {
		return ColumnNameChange{Requested: true}, nil
	}
	name := NormalizeName(request.Name.Value)
	if !IsValidColumnName(name) {
		return ColumnNameChange{}, httpapi.NewValidationError(map[string]string{"name": "invalid"})
	}
	return ColumnNameChange{Requested: true, Name: &name}, nil
}

type MoveColumnRequest struct {
	Position *int `json:"position"`
}

func (request MoveColumnRequest) TargetPosition() (int, error) {
	if request.Position == nil || *request.Position < 0 {
		return 0, httpapi.NewValidationError(map[string]string{"position": "invalid"})
	}
	return *request.Position, nil
}
