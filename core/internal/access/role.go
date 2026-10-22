package access

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

const (
	PermissionInviteCreate = "core.invite.create"
	PermissionInviteRead   = "core.invite.read"
	PermissionInviteRevoke = "core.invite.revoke"
	PermissionUserRead     = "core.user.read"
	PermissionMemberRead   = "core.member.read"
	PermissionMemberManage = "core.member.manage"
)

const RoleNameAdmin = "admin"

var ErrRoleNotAssignable = platform.NewAPIError(http.StatusForbidden, "role_not_assignable")

type Role struct {
	ID          int64
	Token       string
	Name        string
	Permissions []string
}

func (role Role) Grants(permission string) bool {
	for _, granted := range role.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

func (role Role) Covers(other Role) bool {
	for _, permission := range other.Permissions {
		if !role.Grants(permission) {
			return false
		}
	}
	return true
}
