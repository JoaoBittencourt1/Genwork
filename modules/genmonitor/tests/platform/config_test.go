package platform_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/platform"
)

func validServeEnv() map[string]string {
	return map[string]string{
		"GENMONITOR_HTTP_ADDR":  ":8091",
		"CORE_INTERNAL_URL":     "http://core:8484/",
		"GENWORK_MODULE_SECRET": "module-secret-0123456789abcdef",
		"GENWORK_PUBLIC_URL":    "http://localhost:8487",
		"GENMONITOR_HOST_PROC":  "/host/proc",
		"GENMONITOR_HOST_ROOT":  "/host/root",
	}
}

func getenvFrom(values map[string]string) platform.Getenv {
	return func(name string) string { return values[name] }
}

func TestLoadServeConfigAcceptsACompleteEnvironment(t *testing.T) {
	config, err := platform.LoadServeConfig(getenvFrom(validServeEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := platform.ServeConfig{
		HTTPAddr: ":8091", CoreInternalURL: "http://core:8484", ModuleSecret: "module-secret-0123456789abcdef",
		PublicURL: "http://localhost:8487", HostProcPath: "/host/proc", HostRootPath: "/host/root",
	}
	if config != want {
		t.Errorf("config = %+v", config)
	}
}

func TestLoadServeConfigReportsEachMissingVariable(t *testing.T) {
	for name := range validServeEnv() {
		t.Run(name, func(t *testing.T) {
			env := validServeEnv()
			delete(env, name)
			if _, err := platform.LoadServeConfig(getenvFrom(env)); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("error = %v, want mention of %s", err, name)
			}
		})
	}
}

func TestLoadServeConfigRejectsInvalidValues(t *testing.T) {
	cases := map[string]string{"CORE_INTERNAL_URL": "core:8484", "GENWORK_PUBLIC_URL": "ftp://localhost", "GENWORK_MODULE_SECRET": "short"}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			env := validServeEnv()
			env[name] = value
			if _, err := platform.LoadServeConfig(getenvFrom(env)); err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("error = %v, want mention of %s", err, name)
			}
		})
	}
}
