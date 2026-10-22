package members

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

type MemberStatus string

const (
	MemberStatusActive      MemberStatus = "active"
	MemberStatusDeactivated MemberStatus = "deactivated"
)

func ParseMemberStatus(raw string) (MemberStatus, bool) {
	status := MemberStatus(raw)
	switch status {
	case MemberStatusActive, MemberStatusDeactivated:
		return status, true
	}
	return "", false
}

var (
	ErrMemberNotFound      = platform.NewAPIError(http.StatusNotFound, "member_not_found")
	ErrCannotManageSelf    = platform.NewAPIError(http.StatusForbidden, "cannot_manage_self")
	ErrMemberNotManageable = platform.NewAPIError(http.StatusForbidden, "member_not_manageable")
	ErrUnknownModule       = platform.NewValidationError(map[string]string{"restrictedModules": "unknown_module"})
)

// Member is a user who has not been removed, seen with their role and the modules they may not access.
type Member struct {
	User              users.User
	Role              access.Role
	RestrictedModules []string
}

// Status is derived from the deactivation instant, so no stored status can disagree with it.
func (member Member) Status() MemberStatus {
	if member.User.IsDeactivated() {
		return MemberStatusDeactivated
	}
	return MemberStatusActive
}
