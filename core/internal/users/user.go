package users

import (
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const MaxNameLength = 120

type AvatarColor string

const (
	AvatarColorBlue       AvatarColor = "blue"
	AvatarColorLightGreen AvatarColor = "light-green"
	AvatarColorPink       AvatarColor = "pink"
	AvatarColorPurple     AvatarColor = "purple"
	AvatarColorRed        AvatarColor = "red"
	AvatarColorYellow     AvatarColor = "yellow"
	AvatarColorWhite      AvatarColor = "white"
	AvatarColorOrange     AvatarColor = "orange"
)

var AvatarColors = []AvatarColor{
	AvatarColorBlue, AvatarColorLightGreen, AvatarColorPink, AvatarColorPurple,
	AvatarColorRed, AvatarColorYellow, AvatarColorWhite, AvatarColorOrange,
}

var (
	ErrEmailAlreadyRegistered = platform.NewAPIError(http.StatusConflict, "email_already_registered")
	ErrDirectoryUserNotFound  = platform.NewAPIError(http.StatusNotFound, "user_not_found")
)

type User struct {
	ID               int64
	Token            string
	Email            string
	Name             string
	PasswordHash     string
	RoleID           int64
	FailedLoginCount int
	LockedUntil      *time.Time
	LastLoginAt      *time.Time
	CreatedAt        time.Time
	DeactivatedAt    *time.Time
}

func (user User) IsLockedAt(now time.Time) bool {
	return user.LockedUntil != nil && now.Before(*user.LockedUntil)
}

func (user User) IsDeactivated() bool {
	return user.DeactivatedAt != nil
}

// AvatarColor is drawn from the instant the user was created, so it needs no column and never changes.
func (user User) AvatarColor() AvatarColor {
	return AvatarColors[user.CreatedAt.UnixMicro()%int64(len(AvatarColors))]
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func IsValidEmail(normalizedEmail string) bool {
	address, err := mail.ParseAddress(normalizedEmail)
	return err == nil && address.Address == normalizedEmail && address.Name == ""
}

func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

func IsValidName(normalizedName string) bool {
	length := utf8.RuneCountInString(normalizedName)
	return length > 0 && length <= MaxNameLength
}
