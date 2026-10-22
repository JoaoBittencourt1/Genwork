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

var deactivationTime = time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)

func TestUpdateRoleChangesOnlyTheRole(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	admin, err := access.RoleRepository{}.FindByName(ctx, db, "admin")
	if err != nil {
		t.Fatalf("find admin role: %v", err)
	}

	if err := (users.Repository{}).UpdateRole(ctx, db, user.ID, admin.ID); err != nil {
		t.Fatalf("update role: %v", err)
	}

	updated, err := users.Repository{}.FindByID(ctx, db, user.ID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if updated.RoleID != admin.ID || updated.Email != user.Email || updated.Name != user.Name || updated.DeactivatedAt != nil {
		t.Errorf("updated user = %+v", updated)
	}
}

func TestUpdateDeactivatedAtSetsAndClearsTheStatus(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	repository := users.Repository{}

	if err := repository.UpdateDeactivatedAt(ctx, db, user.ID, &deactivationTime); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	deactivated, err := repository.FindByID(ctx, db, user.ID)
	if err != nil {
		t.Fatalf("find deactivated user: %v", err)
	}
	if !deactivated.IsDeactivated() || !deactivated.DeactivatedAt.Equal(deactivationTime) {
		t.Errorf("deactivated user = %+v", deactivated)
	}

	if err := repository.UpdateDeactivatedAt(ctx, db, user.ID, nil); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	reactivated, err := repository.FindByID(ctx, db, user.ID)
	if err != nil {
		t.Fatalf("find reactivated user: %v", err)
	}
	if reactivated.IsDeactivated() {
		t.Errorf("reactivated user = %+v", reactivated)
	}
}

func TestSoftDeleteHidesUserAndFreesEmail(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	repository := users.Repository{}

	if err := repository.SoftDelete(ctx, db, user.ID, deactivationTime); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if _, err := repository.FindByID(ctx, db, user.ID); !errors.Is(err, users.ErrUserNotFound) {
		t.Errorf("find removed user error = %v", err)
	}
	userstest.CreateUser(t, db, "ana@example.com", "member")
}

func TestUpdatesOfRemovedUserReportNotFound(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	repository := users.Repository{}
	if err := repository.SoftDelete(ctx, db, user.ID, deactivationTime); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	updates := map[string]error{
		"update role":       repository.UpdateRole(ctx, db, user.ID, user.RoleID),
		"deactivate":        repository.UpdateDeactivatedAt(ctx, db, user.ID, &deactivationTime),
		"soft delete again": repository.SoftDelete(ctx, db, user.ID, deactivationTime),
	}
	for name, err := range updates {
		if !errors.Is(err, users.ErrUserNotFound) {
			t.Errorf("%s error = %v, want ErrUserNotFound", name, err)
		}
	}
}

func TestListNotRemovedIncludesDeactivatedUsers(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := users.Repository{}
	deactivated := userstest.CreateUser(t, db, "bruno@example.com", "member")
	removed := userstest.CreateUser(t, db, "carla@example.com", "member")
	userstest.CreateUser(t, db, "ana@example.com", "admin")
	if err := repository.UpdateDeactivatedAt(ctx, db, deactivated.ID, &deactivationTime); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if err := repository.SoftDelete(ctx, db, removed.ID, deactivationTime); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	listed, err := repository.ListNotRemoved(ctx, db)

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 2 || listed[0].Email != "ana@example.com" || listed[1].Email != "bruno@example.com" || !listed[1].IsDeactivated() {
		t.Errorf("listed users = %+v", listed)
	}
}
