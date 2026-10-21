package labels

import "github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"

type CreateLabelRequest struct {
	Name  string     `json:"name"`
	Color LabelColor `json:"color"`
}

type LabelInput struct {
	Name  string
	Color LabelColor
}

func (request CreateLabelRequest) Validate() (LabelInput, error) {
	fields := map[string]string{}
	name := NormalizeName(request.Name)
	if !IsValidName(name) {
		fields["name"] = "invalid"
	}
	if !request.Color.IsValid() {
		fields["color"] = "invalid"
	}
	if len(fields) > 0 {
		return LabelInput{}, httpapi.NewValidationError(fields)
	}
	return LabelInput{Name: name, Color: request.Color}, nil
}

type UpdateLabelRequest struct {
	Name  httpapi.PatchField[string]     `json:"name"`
	Color httpapi.PatchField[LabelColor] `json:"color"`
}

// LabelChanges holds only the fields a PATCH sent; neither field is nullable (rule 17).
type LabelChanges struct {
	Name  *string
	Color *LabelColor
}

func (request UpdateLabelRequest) Validate() (LabelChanges, error) {
	fields := map[string]string{}
	var changes LabelChanges
	if request.Name.Present {
		name := NormalizeName(request.Name.Value)
		if request.Name.Null || !IsValidName(name) {
			fields["name"] = "invalid"
		}
		changes.Name = &name
	}
	if request.Color.Present {
		if request.Color.Null || !request.Color.Value.IsValid() {
			fields["color"] = "invalid"
		}
		changes.Color = &request.Color.Value
	}
	if len(fields) > 0 {
		return LabelChanges{}, httpapi.NewValidationError(fields)
	}
	return changes, nil
}

func (changes LabelChanges) ApplyTo(label Label) Label {
	if changes.Name != nil {
		label.Name = *changes.Name
	}
	if changes.Color != nil {
		label.Color = *changes.Color
	}
	return label
}
