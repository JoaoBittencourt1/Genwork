package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/jackc/pgx/v5/pgxpool"
)

var restrictionTime = time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

// registerModule registers a module whose permissions are all granted to the member role by default.
func registerModule(t *testing.T, db *pgxpool.Pool, name string, permissionNames ...string) {
	t.Helper()
	declarations := make([]modules.PermissionDeclaration, 0, len(permissionNames))
	for _, permissionName := range permissionNames {
		declarations = append(declarations, modules.PermissionDeclaration{Name: permissionName, DefaultRoles: []string{"member"}})
	}
	_, err := modules.NewModuleService(db, "1.0.0", func() time.Time { return restrictionTime }).Register(context.Background(), modules.Registration{
		ModuleName:  name,
		Version:     "0.1.0",
		CoreRange:   ">=1.0.0 <2.0.0",
		APIPrefix:   "/api/" + name + "/v1",
		Permissions: declarations,
	})
	if err != nil {
		t.Fatalf("register module %s: %v", name, err)
	}
}
