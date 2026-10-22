package members

import (
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

type MemberResponse struct {
	Token             string                     `json:"token"`
	Name              string                     `json:"name"`
	Email             string                     `json:"email"`
	AvatarColor       users.AvatarColor          `json:"avatarColor"`
	Role              access.RoleSummaryResponse `json:"role"`
	Status            MemberStatus               `json:"status"`
	JoinedAt          time.Time                  `json:"joinedAt"`
	LastSignInAt      *time.Time                 `json:"lastSignInAt"`
	RestrictedModules []string                   `json:"restrictedModules"`
}

type MemberListResponse struct {
	Members []MemberResponse `json:"members"`
}

func NewMemberResponse(member Member) MemberResponse {
	restrictedModules := member.RestrictedModules
	if restrictedModules == nil {
		restrictedModules = []string{}
	}
	return MemberResponse{
		Token:             member.User.Token,
		Name:              member.User.Name,
		Email:             member.User.Email,
		AvatarColor:       member.User.AvatarColor(),
		Role:              access.RoleSummaryResponse{Token: member.Role.Token, Name: member.Role.Name},
		Status:            member.Status(),
		JoinedAt:          member.User.CreatedAt,
		LastSignInAt:      member.User.LastLoginAt,
		RestrictedModules: restrictedModules,
	}
}
