package boards

import "github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"

type CreateBoardRequest struct {
	Name       string  `json:"name"`
	TaskPrefix *string `json:"taskPrefix"`
}

// CreateBoardInput.TaskPrefix is nil when the client leaves the choice to the server.
type CreateBoardInput struct {
	Name       string
	TaskPrefix *string
}

func (request CreateBoardRequest) Validate() (CreateBoardInput, error) {
	fields := map[string]string{}
	input := CreateBoardInput{Name: NormalizeName(request.Name)}
	if !IsValidBoardName(input.Name) {
		fields["name"] = "invalid"
	}
	if request.TaskPrefix != nil {
		prefix := NormalizeTaskPrefix(*request.TaskPrefix)
		if !IsValidTaskPrefix(prefix) {
			fields["taskPrefix"] = "invalid"
		}
		input.TaskPrefix = &prefix
	}
	if len(fields) > 0 {
		return CreateBoardInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}

type UpdateBoardRequest struct {
	Name       httpapi.PatchField[string] `json:"name"`
	TaskPrefix httpapi.PatchField[string] `json:"taskPrefix"`
}

// BoardChanges keeps the PATCH semantics of rule 17: a nil pointer means "leave unchanged"; neither field is nullable.
type BoardChanges struct {
	Name       *string
	TaskPrefix *string
}

func (request UpdateBoardRequest) Validate() (BoardChanges, error) {
	fields := map[string]string{}
	var changes BoardChanges
	if request.Name.Present {
		name := NormalizeName(request.Name.Value)
		if request.Name.Null || !IsValidBoardName(name) {
			fields["name"] = "invalid"
		}
		changes.Name = &name
	}
	if request.TaskPrefix.Present {
		prefix := NormalizeTaskPrefix(request.TaskPrefix.Value)
		if request.TaskPrefix.Null || !IsValidTaskPrefix(prefix) {
			fields["taskPrefix"] = "invalid"
		}
		changes.TaskPrefix = &prefix
	}
	if len(fields) > 0 {
		return BoardChanges{}, httpapi.NewValidationError(fields)
	}
	return changes, nil
}

func (changes BoardChanges) IsEmpty() bool {
	return changes.Name == nil && changes.TaskPrefix == nil
}

func (changes BoardChanges) ApplyTo(board Board) Board {
	if changes.Name != nil {
		board.Name = *changes.Name
	}
	if changes.TaskPrefix != nil {
		board.TaskPrefix = *changes.TaskPrefix
	}
	return board
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
