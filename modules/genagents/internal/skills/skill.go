package skills

import (
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
	MaxFiles             = 20
	MaxFileBytes         = 256 * 1024
	SkillFileName        = "SKILL.md"
)

var namePattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

var (
	ErrSkillNotFound  = httpapi.NewAPIError(http.StatusNotFound, "skill_not_found")
	ErrSkillNameTaken = httpapi.NewAPIError(http.StatusConflict, "skill_name_taken")
)

type SkillFile struct {
	Path    string
	Content string
}

// Skill is an Agent Skills package: a SKILL.md body with name and description, plus extra text files. Version grows
// with every edit, so a run records which version it started with.
type Skill struct {
	ID          int64
	Token       string
	Name        string
	Description string
	Body        string
	Version     int
	Files       []SkillFile
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func IsValidName(name string) bool {
	return namePattern.MatchString(name)
}

func IsValidDescription(description string) bool {
	length := utf8.RuneCountInString(description)
	return length > 0 && length <= MaxDescriptionLength
}

func IsValidBody(body string) bool {
	return strings.TrimSpace(body) != "" && len(body) <= MaxFileBytes
}

// IsValidFilePath accepts only clean relative paths that stay inside the skill directory.
func IsValidFilePath(filePath string) bool {
	if filePath == "" || len(filePath) > 200 || strings.ContainsAny(filePath, "\\\x00") || strings.HasPrefix(filePath, "/") {
		return false
	}
	if path.Clean(filePath) != filePath || filePath == SkillFileName {
		return false
	}
	for _, segment := range strings.Split(filePath, "/") {
		if segment == ".." || segment == "." || segment == "" {
			return false
		}
	}
	return true
}

// AreValidFiles checks the count, size, and uniqueness of paths of the extra files.
func AreValidFiles(files []SkillFile) bool {
	if len(files) > MaxFiles {
		return false
	}
	seen := map[string]bool{}
	for _, file := range files {
		if !IsValidFilePath(file.Path) || len(file.Content) > MaxFileBytes || seen[file.Path] {
			return false
		}
		seen[file.Path] = true
	}
	return true
}

// ParseSkillMarkdown reads a SKILL.md that starts with front matter holding name and description.
func ParseSkillMarkdown(content string) (name, description, body string, valid bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	rest, found := strings.CutPrefix(normalized, "---\n")
	if !found {
		return "", "", "", false
	}
	header, body, found := strings.Cut(rest, "\n---")
	if !found {
		return "", "", "", false
	}
	body = strings.TrimPrefix(body, "\n")
	for _, line := range strings.Split(header, "\n") {
		key, value, hasValue := strings.Cut(line, ":")
		if !hasValue {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	return name, description, strings.TrimLeft(body, "\n"), name != "" && description != ""
}
