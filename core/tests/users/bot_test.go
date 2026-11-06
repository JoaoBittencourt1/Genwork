package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users/userstest"
)

func TestExistingMembersAreHumans(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	user := userstest.CreateUser(t, db, "ana@example.com", "member")

	if user.Kind != users.MemberKindHuman || user.IsBot() {
		t.Errorf("kind = %q", user.Kind)
	}
}

func TestBotRequiresNoEmailOrPassword(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	role, err := access.RoleRepository{}.FindByName(ctx, db, "member")
	if err != nil {
		t.Fatalf("find role: %v", err)
	}

	bot, err := users.Repository{}.CreateBot(ctx, db, users.NewBot{Name: "Dev Bot", RoleID: role.ID})
	if err != nil {
		t.Fatalf("create bot: %v", err)
	}

	if !bot.IsBot() || bot.Email != "" || bot.PasswordHash != "" || bot.Name != "Dev Bot" {
		t.Errorf("bot = %+v", bot)
	}
}

func TestHumanStillRequiresEmailAndPassword(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	role, _ := access.RoleRepository{}.FindByName(ctx, db, "member")

	_, err := db.Exec(ctx, `INSERT INTO users (name, role_id) VALUES ('Ana', $1)`, role.ID)

	if err == nil {
		t.Error("a human without e-mail and password was accepted")
	}
}

func TestEmailUniquenessIgnoresBots(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	userstest.CreateUser(t, db, "ana@example.com", "member")
	role, _ := access.RoleRepository{}.FindByName(ctx, db, "member")

	for _, name := range []string{"Bot A", "Bot B"} {
		if _, err := (users.Repository{}).CreateBot(ctx, db, users.NewBot{Name: name, RoleID: role.ID}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	_, err := users.Repository{}.Create(ctx, db, users.NewUser{Email: "ana@example.com", Name: "Ana 2", PasswordHash: "x", RoleID: role.ID})

	if !errors.Is(err, users.ErrEmailAlreadyRegistered) {
		t.Errorf("duplicate human e-mail error = %v", err)
	}
}

func TestBotIsNotFoundByEmail(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	role, _ := access.RoleRepository{}.FindByName(ctx, db, "member")
	if _, err := (users.Repository{}).CreateBot(ctx, db, users.NewBot{Name: "Dev Bot", RoleID: role.ID}); err != nil {
		t.Fatalf("create bot: %v", err)
	}

	if _, err := (users.Repository{}).FindByEmail(ctx, db, "dev bot"); !errors.Is(err, users.ErrUserNotFound) {
		t.Errorf("find bot by e-mail error = %v", err)
	}
}

func TestBotCredentialStoresOnlyTheHashAndRotates(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	role, _ := access.RoleRepository{}.FindByName(ctx, db, "member")
	bot, _ := users.Repository{}.CreateBot(ctx, db, users.NewBot{Name: "Dev Bot", RoleID: role.ID})
	repository := users.BotCredentialRepository{}

	if _, err := repository.Create(ctx, db, bot.ID, users.HashBotSecret("first-secret")); err != nil {
		t.Fatalf("create credential: %v", err)
	}
	found, err := repository.FindActiveByBotToken(ctx, db, bot.Token)
	if err != nil {
		t.Fatalf("find credential: %v", err)
	}
	if string(found.SecretHash) == "first-secret" || !found.Matches("first-secret") || found.Matches("other") {
		t.Errorf("credential = %+v", found)
	}

	rotatedAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	if err := repository.Rotate(ctx, db, bot.ID, users.HashBotSecret("second-secret"), rotatedAt); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	rotated, _ := repository.FindActiveByBotToken(ctx, db, bot.Token)
	if rotated.Matches("first-secret") || !rotated.Matches("second-secret") || rotated.RotatedAt == nil {
		t.Errorf("rotated credential = %+v", rotated)
	}
}

func TestDeactivatedBotHasNoActiveCredential(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	role, _ := access.RoleRepository{}.FindByName(ctx, db, "member")
	bot, _ := users.Repository{}.CreateBot(ctx, db, users.NewBot{Name: "Dev Bot", RoleID: role.ID})
	repository := users.BotCredentialRepository{}
	if _, err := repository.Create(ctx, db, bot.ID, users.HashBotSecret("secret")); err != nil {
		t.Fatalf("create credential: %v", err)
	}
	if err := (users.Repository{}).UpdateDeactivatedAt(ctx, db, bot.ID, &deactivationTime); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if _, err := repository.FindActiveByBotToken(ctx, db, bot.Token); !errors.Is(err, users.ErrBotCredentialNotFound) {
		t.Errorf("find credential of deactivated bot error = %v", err)
	}
}
