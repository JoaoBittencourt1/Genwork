package skills

import (
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type SkillFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type CreateSkillRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Body        string             `json:"body"`
	Files       []SkillFileRequest `json:"files"`
}

func (request CreateSkillRequest) Validate() (Skill, error) {
	skill := Skill{
		Name: strings.TrimSpace(request.Name), Description: strings.TrimSpace(request.Description), Body: request.Body,
		Files: toSkillFiles(request.Files),
	}
	return skill, validateSkill(skill)
}

func validateSkill(skill Skill) error {
	fields := map[string]string{}
	if !IsValidName(skill.Name) {
		fields["name"] = "invalid"
	}
	if !IsValidDescription(skill.Description) {
		fields["description"] = "invalid"
	}
	if !IsValidBody(skill.Body) {
		fields["body"] = "invalid"
	}
	if !AreValidFiles(skill.Files) {
		fields["files"] = "invalid"
	}
	if len(fields) > 0 {
		return httpapi.NewValidationError(fields)
	}
	return nil
}

func toSkillFiles(requested []SkillFileRequest) []SkillFile {
	files := make([]SkillFile, 0, len(requested))
	for _, file := range requested {
		files = append(files, SkillFile{Path: file.Path, Content: file.Content})
	}
	return files
}

type ImportSkillRequest struct {
	Content string `json:"content"`
}

// Validate fills the name and description from the front matter and keeps the rest as the body.
func (request ImportSkillRequest) Validate() (Skill, error) {
	name, description, body, parsed := ParseSkillMarkdown(request.Content)
	if !parsed {
		return Skill{}, httpapi.NewValidationError(map[string]string{"content": "invalid_front_matter"})
	}
	skill := Skill{Name: name, Description: description, Body: body}
	return skill, validateSkill(skill)
}

// UpdateSkillRequest is a PATCH body: omitted fields stay unchanged and none accepts null (rule 17).
type UpdateSkillRequest struct {
	Name        httpapi.PatchField[string]             `json:"name"`
	Description httpapi.PatchField[string]             `json:"description"`
	Body        httpapi.PatchField[string]             `json:"body"`
	Files       httpapi.PatchField[[]SkillFileRequest] `json:"files"`
}

// ApplyTo returns the skill with the present fields changed, validated as a whole.
func (request UpdateSkillRequest) ApplyTo(skill Skill) (Skill, error) {
	fields := map[string]string{}
	for name, null := range map[string]bool{"name": request.Name.Null, "description": request.Description.Null, "body": request.Body.Null, "files": request.Files.Null} {
		if null {
			fields[name] = "not_nullable"
		}
	}
	if len(fields) > 0 {
		return Skill{}, httpapi.NewValidationError(fields)
	}
	if request.Name.Present {
		skill.Name = strings.TrimSpace(request.Name.Value)
	}
	if request.Description.Present {
		skill.Description = strings.TrimSpace(request.Description.Value)
	}
	if request.Body.Present {
		skill.Body = request.Body.Value
	}
	if request.Files.Present {
		skill.Files = toSkillFiles(request.Files.Value)
	}
	return skill, validateSkill(skill)
}
