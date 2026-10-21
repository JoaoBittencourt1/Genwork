package command_test

import (
	"context"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/command"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
)

var fastParams = users.PasswordParams{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestParseAdminCreateOptionsRequiresEveryFlag(t *testing.T) {
	valid := []string{"--email", "ana@example.com", "--name", "Ana", "--password-stdin"}
	if _, err := command.ParseAdminCreateOptions(valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, args := range [][]string{
		{"--email", "ana@example.com", "--name", "Ana"},
		{"--name", "Ana", "--password-stdin"},
		{"--email", "ana@example.com", "--password-stdin"},
		{"--password", "inline", "--email", "ana@example.com", "--name", "Ana"},
		{"--email", "ana@example.com", "--name", "  ", "--password-stdin"},
	} {
		if _, err := command.ParseAdminCreateOptions(args); err == nil {
			t.Errorf("expected %v to be rejected", args)
		}
	}
}

func TestReadPasswordLineStripsLineEnding(t *testing.T) {
	for input, expected := range map[string]string{
		"correct horse battery\n":   "correct horse battery",
		"correct horse battery\r\n": "correct horse battery",
		"correct horse battery":     "correct horse battery",
	} {
		got, err := command.ReadPasswordLine(strings.NewReader(input))
		if err != nil || got != expected {
			t.Errorf("command.ReadPasswordLine(%q) = %q, %v", input, got, err)
		}
	}
}

func TestCreateAdministrator(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()

	user, err := command.CreateAdministrator(ctx, db, fastParams, "Ana@Example.com", "Ana", "correct horse battery")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	role, _ := access.RoleRepository{}.FindByID(ctx, db, user.RoleID)
	if role.Name != access.RoleNameAdmin || user.Email != "ana@example.com" {
		t.Errorf("user = %+v, role = %s", user, role.Name)
	}
	matches, _, err := users.NewPasswordHasher(fastParams, 1).Verify("correct horse battery", user.PasswordHash)
	if err != nil || !matches {
		t.Error("the administrator must be able to sign in with the chosen password")
	}

	if _, err := command.CreateAdministrator(ctx, db, fastParams, "ana@example.com", "Ana", "correct horse battery"); err != users.ErrEmailAlreadyRegistered {
		t.Errorf("duplicate e-mail error = %v", err)
	}
	if _, err := command.CreateAdministrator(ctx, db, fastParams, "bia@example.com", "Bia", "short"); err != users.ErrPasswordLength {
		t.Errorf("short password error = %v", err)
	}
}

func TestDescribeAdminCreateErrorExplainsCodes(t *testing.T) {
	cases := map[error]string{
		users.ErrPasswordLength:         "password_length_invalid: the password must be between 8 and 128 characters",
		users.ErrEmailAlreadyRegistered: "email_already_registered: a user with this e-mail already exists",
		platform.NewValidationError(map[string]string{"name": "invalid", "email": "invalid"}): "validation_failed: invalid email, name",
	}
	for input, expected := range cases {
		if got := command.DescribeAdminCreateError(input).Error(); got != expected {
			t.Errorf("command.DescribeAdminCreateError(%v) = %q, want %q", input, got, expected)
		}
	}
}
