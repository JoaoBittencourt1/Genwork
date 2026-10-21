package labels

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const MaxLabelNameLength = 40

var (
	ErrLabelNotFound  = httpapi.NewAPIError(http.StatusNotFound, "label_not_found")
	ErrLabelNameTaken = httpapi.NewAPIError(http.StatusConflict, "label_name_taken")
)

// LabelColor is a palette entry; clients map it to a theme variable (rule 81).
type LabelColor string

const (
	ColorGray   LabelColor = "gray"
	ColorRed    LabelColor = "red"
	ColorOrange LabelColor = "orange"
	ColorYellow LabelColor = "yellow"
	ColorGreen  LabelColor = "green"
	ColorTeal   LabelColor = "teal"
	ColorBlue   LabelColor = "blue"
	ColorPurple LabelColor = "purple"
	ColorPink   LabelColor = "pink"
)

var Palette = []LabelColor{ColorGray, ColorRed, ColorOrange, ColorYellow, ColorGreen, ColorTeal, ColorBlue, ColorPurple, ColorPink}

func (color LabelColor) IsValid() bool {
	for _, candidate := range Palette {
		if candidate == color {
			return true
		}
	}
	return false
}

type Label struct {
	ID        int64
	Token     string
	BoardID   int64
	Name      string
	Color     LabelColor
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

func IsValidName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxLabelNameLength
}
