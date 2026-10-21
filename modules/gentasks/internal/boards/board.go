package boards

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxBoardNameLength  = 80
	MaxColumnNameLength = 60
)

var (
	ErrBoardNotFound  = httpapi.NewAPIError(http.StatusNotFound, "board_not_found")
	ErrColumnNotFound = httpapi.NewAPIError(http.StatusNotFound, "column_not_found")
	ErrColumnNotEmpty = httpapi.NewAPIError(http.StatusConflict, "column_not_empty")
	ErrLastColumn     = httpapi.NewAPIError(http.StatusConflict, "last_column")

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

type Board struct {
	ID        int64
	Token     string
	Name      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
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
