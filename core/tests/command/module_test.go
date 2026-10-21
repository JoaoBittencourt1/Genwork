package command_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/command"
	"github.com/JoaoBittencourt1/Genwork/core/internal/discovery"
	"github.com/JoaoBittencourt1/Genwork/core/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
)

func registeredGentasks(t *testing.T) (*modules.ModuleService, string, platform.Getenv) {
	t.Helper()
	pool := dbtest.NewMigratedSchema(t)
	service := modules.NewModuleService(pool, discovery.CoreAPIVersion, time.Now)
	registration := modules.Registration{ModuleName: "gentasks", Version: "0.1.0", CoreRange: ">=1.0.0", APIPrefix: "/api/gentasks/v1"}
	if _, err := service.Register(context.Background(), registration); err != nil {
		t.Fatalf("register: %v", err)
	}
	env := envFrom(map[string]string{"DATABASE_URL": dbtest.DatabaseURL(t)})
	return service, pool.Config().ConnConfig.RuntimeParams["search_path"], env
}

func enabledModuleNames(t *testing.T, service *modules.ModuleService) []string {
	t.Helper()
	enabled, err := service.ListEnabled(context.Background())
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	var names []string
	for _, module := range enabled {
		names = append(names, module.Name)
	}
	return names
}

func TestModuleDisableAndEnable(t *testing.T) {
	service, schema, env := registeredGentasks(t)
	var output bytes.Buffer

	if err := command.RunModuleState(context.Background(), env, schema, []string{"disable", "gentasks"}, &output); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if names := enabledModuleNames(t, service); len(names) != 0 {
		t.Fatalf("enabled after disable = %v", names)
	}
	if _, err := service.Register(context.Background(), modules.Registration{ModuleName: "gentasks", Version: "0.1.1", CoreRange: ">=1.0.0", APIPrefix: "/api/gentasks/v1"}); err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if names := enabledModuleNames(t, service); len(names) != 0 {
		t.Fatalf("re-registration must keep the module disabled, enabled = %v", names)
	}

	if err := command.RunModuleState(context.Background(), env, schema, []string{"enable", "gentasks"}, &output); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if names := enabledModuleNames(t, service); len(names) != 1 || names[0] != "gentasks" {
		t.Errorf("enabled after enable = %v", names)
	}
	if !strings.Contains(output.String(), "module gentasks disabled") || !strings.Contains(output.String(), "module gentasks enabled") {
		t.Errorf("output = %q", output.String())
	}
}

func TestModuleStateForUnknownModuleIsANoOp(t *testing.T) {
	_, schema, env := registeredGentasks(t)
	var output bytes.Buffer

	if err := command.RunModuleState(context.Background(), env, schema, []string{"disable", "reports"}, &output); err != nil {
		t.Fatalf("unknown module must not fail: %v", err)
	}
	if !strings.Contains(output.String(), "nothing to change") {
		t.Errorf("output = %q", output.String())
	}
}

func TestModuleStateRequiresActionAndID(t *testing.T) {
	for _, args := range [][]string{nil, {"disable"}, {"pause", "gentasks"}, {"enable", "gentasks", "extra"}} {
		if err := command.RunModuleState(context.Background(), envFrom(nil), "unused", args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%v: error = %v", args, err)
		}
	}
	if err := command.Run(context.Background(), []string{"module"}, envFrom(nil), nil, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("Run without a module action: error = %v", err)
	}
	if err := command.RunModuleState(context.Background(), envFrom(nil), "unused", []string{"enable", "gentasks"}, &bytes.Buffer{}); err == nil {
		t.Error("a missing DATABASE_URL must fail")
	}
}
