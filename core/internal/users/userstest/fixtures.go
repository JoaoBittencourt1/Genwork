package userstest

import (
	"context"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

// unusablePasswordHash keeps fixtures fast: no test signs in with these users, so no real Argon2id hash is needed.
const unusablePasswordHash = "$argon2id$v=19$m=8192,t=1,p=1$c2FsdA$a2V5"

func CreateUser(t *testing.T, db platform.DBTX, email, roleName string) users.User {
	t.Helper()
	ctx := context.Background()
	role, err := access.RoleRepository{}.FindByName(ctx, db, roleName)
	if err != nil {
		t.Fatalf("find role %s: %v", roleName, err)
	}
	user, err := users.Repository{}.Create(ctx, db, users.NewUser{
		Email: email, Name: email, PasswordHash: unusablePasswordHash, RoleID: role.ID,
	})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return user
}
