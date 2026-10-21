package boards

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxBoardNameLength  = 80
	MaxColumnNameLength = 60

	derivedTaskPrefixLength = 3
	fallbackTaskPrefix      = "TASK"
)

// taskPrefixPattern: an uppercase letter, then up to nine uppercase letters or digits, so "PREFIX-12" stays unambiguous.
var taskPrefixPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}$`)

var (
	ErrBoardNotFound   = httpapi.NewAPIError(http.StatusNotFound, "board_not_found")
	ErrColumnNotFound  = httpapi.NewAPIError(http.StatusNotFound, "column_not_found")
	ErrColumnNotEmpty  = httpapi.NewAPIError(http.StatusConflict, "column_not_empty")
	ErrLastColumn      = httpapi.NewAPIError(http.StatusConflict, "last_column")
	ErrTaskPrefixTaken = httpapi.NewAPIError(http.StatusConflict, "task_prefix_taken")

	errCustomColumnNeedsName = httpapi.NewValidationError(map[string]string{"name": "required"})
)

type ColumnPreset string

const (
	PresetBacklog ColumnPreset = "backlog"
	PresetTodo    ColumnPreset = "todo"
	PresetDone    ColumnPreset = "done"
)

// DefaultColumnPresets is the workflow every new board starts with, in order.
var DefaultColumnPresets = []ColumnPreset{PresetBacklog, PresetTodo, PresetDone}

// Board.TaskPrefix is unique among active boards; a task's identifier is the prefix followed by the task number.
type Board struct {
	ID         int64
	Token      string
	Name       string
	TaskPrefix string
	CreatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Column has a preset, a custom name, or both; clients show the localized preset label while Name is nil.
type Column struct {
	ID        int64
	Token     string
	BoardID   int64
	Preset    *ColumnPreset
	Name      *string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

func IsValidBoardName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxBoardNameLength
}

func IsValidColumnName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxColumnNameLength
}

func NormalizeTaskPrefix(prefix string) string {
	return strings.ToUpper(strings.TrimSpace(prefix))
}

func IsValidTaskPrefix(normalizedPrefix string) bool {
	return taskPrefixPattern.MatchString(normalizedPrefix)
}

// DeriveTaskPrefix suggests a prefix from the first ASCII letters and digits of a board name; the migration that
// introduced prefixes applies the same rule to existing boards.
func DeriveTaskPrefix(name string) string {
	var prefix strings.Builder
	for _, character := range name {
		if character >= 'a' && character <= 'z' {
			character -= 'a' - 'A'
		}
		isLetter := character >= 'A' && character <= 'Z'
		isDigit := character >= '0' && character <= '9'
		if isLetter || (isDigit && prefix.Len() > 0) {
			prefix.WriteRune(character)
		}
		if prefix.Len() == derivedTaskPrefixLength {
			break
		}
	}
	if prefix.Len() == 0 {
		return fallbackTaskPrefix
	}
	return prefix.String()
}

// FirstFreeTaskPrefix returns base, or base followed by the smallest suffix from 2 that no active board uses.
func FirstFreeTaskPrefix(base string, taken []string) string {
	takenSet := make(map[string]bool, len(taken))
	for _, prefix := range taken {
		takenSet[prefix] = true
	}
	candidate := base
	for suffix := 2; takenSet[candidate]; suffix++ {
		candidate = base + strconv.Itoa(suffix)
	}
	return candidate
}
