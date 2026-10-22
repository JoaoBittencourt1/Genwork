package access_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users/userstest"
)

func restrictedModuleNames(t *testing.T, db platform.DBTX, userID int64) []string {
	t.Helper()
	byUser, err := access.ModuleRestrictionRepository{}.ListModuleNamesByUserIDs(context.Background(), db, []int64{userID})
	if err != nil {
		t.Fatalf("list restrictions: %v", err)
	}
	return byUser[userID]
}

func countLiftedRestrictions(t *testing.T, db platform.DBTX, userID int64) int {
	t.Helper()
	var lifted int
	err := db.QueryRow(context.Background(),
		`SELECT count(*) FROM user_module_restrictions WHERE user_id = $1 AND deleted_at IS NOT NULL`, userID).Scan(&lifted)
	if err != nil {
		t.Fatalf("count lifted restrictions: %v", err)
	}
	return lifted
}

func TestReplaceModuleRestrictionsReplacesTheWholeSet(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		registerModule(t, db, name)
	}
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	repository := access.ModuleRestrictionRepository{}

	steps := []struct {
		restrict []string
		want     []string
	}{
		{restrict: []string{"beta", "alpha"}, want: []string{"alpha", "beta"}},
		{restrict: []string{"gamma", "beta"}, want: []string{"beta", "gamma"}},
		{restrict: []string{}, want: nil},
	}
	for _, step := range steps {
		if err := repository.Replace(ctx, db, user.ID, step.restrict, restrictionTime); err != nil {
			t.Fatalf("replace with %v: %v", step.restrict, err)
		}
		if got := restrictedModuleNames(t, db, user.ID); !slices.Equal(got, step.want) {
			t.Errorf("after restricting %v: restricted = %v, want %v", step.restrict, got, step.want)
		}
	}
	if lifted := countLiftedRestrictions(t, db, user.ID); lifted != 3 {
		t.Errorf("lifted restrictions = %d, want 3 soft-deleted rows", lifted)
	}
}

func TestReplaceModuleRestrictionsCountsDuplicatesOnce(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	registerModule(t, db, "alpha")
	user := userstest.CreateUser(t, db, "ana@example.com", "member")

	err := access.ModuleRestrictionRepository{}.Replace(context.Background(), db, user.ID, []string{"alpha", "alpha"}, restrictionTime)

	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := restrictedModuleNames(t, db, user.ID); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("restricted = %v", got)
	}
}

func TestReplaceModuleRestrictionsRejectsUnknownModules(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	registerModule(t, db, "alpha")
	user := userstest.CreateUser(t, db, "ana@example.com", "member")
	repository := access.ModuleRestrictionRepository{}
	if err := repository.Replace(ctx, db, user.ID, []string{"alpha"}, restrictionTime); err != nil {
		t.Fatalf("replace: %v", err)
	}

	err := repository.Replace(ctx, db, user.ID, []string{"never-registered"}, restrictionTime)

	if !errors.Is(err, access.ErrUnknownModule) {
		t.Fatalf("error = %v, want ErrUnknownModule", err)
	}
	if got := restrictedModuleNames(t, db, user.ID); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("restricted after rejected replace = %v", got)
	}
}

func TestListModuleNamesByUserIDsBatchesUsers(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	registerModule(t, db, "alpha")
	registerModule(t, db, "beta")
	ana := userstest.CreateUser(t, db, "ana@example.com", "member")
	bruno := userstest.CreateUser(t, db, "bruno@example.com", "member")
	carla := userstest.CreateUser(t, db, "carla@example.com", "member")
	repository := access.ModuleRestrictionRepository{}
	if err := repository.Replace(ctx, db, ana.ID, []string{"beta", "alpha"}, restrictionTime); err != nil {
		t.Fatalf("restrict ana: %v", err)
	}
	if err := repository.Replace(ctx, db, bruno.ID, []string{"beta"}, restrictionTime); err != nil {
		t.Fatalf("restrict bruno: %v", err)
	}

	byUser, err := repository.ListModuleNamesByUserIDs(ctx, db, []int64{ana.ID, bruno.ID, carla.ID})

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !slices.Equal(byUser[ana.ID], []string{"alpha", "beta"}) || !slices.Equal(byUser[bruno.ID], []string{"beta"}) || len(byUser[carla.ID]) != 0 {
		t.Errorf("restrictions by user = %v", byUser)
	}
}
