package users

import (
	"context"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type Service struct {
	hasher *PasswordHasher
	users  Repository
}

func NewService(hasher *PasswordHasher) *Service {
	return &Service{hasher: hasher}
}

type CreateInput struct {
	Email    string
	Name     string
	Password string
	RoleID   int64
}

func ValidateCreateInput(input CreateInput) error {
	fields := map[string]string{}
	if !IsValidEmail(NormalizeEmail(input.Email)) {
		fields["email"] = "invalid"
	}
	if !IsValidName(NormalizeName(input.Name)) {
		fields["name"] = "invalid"
	}
	if len(fields) > 0 {
		return platform.NewValidationError(fields)
	}
	return ValidatePasswordLength(input.Password)
}

// Create runs on the caller's connection or transaction so it can join larger units of work such as invite acceptance.
func (service *Service) Create(ctx context.Context, db platform.DBTX, input CreateInput) (User, error) {
	if err := ValidateCreateInput(input); err != nil {
		return User{}, err
	}
	return service.users.Create(ctx, db, NewUser{
		Email:        NormalizeEmail(input.Email),
		Name:         NormalizeName(input.Name),
		PasswordHash: service.hasher.Hash(input.Password),
		RoleID:       input.RoleID,
	})
}
