package platform_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
)

func validServeEnv() map[string]string {
	return map[string]string{
		"GENTASKS_DATABASE_URL": "postgres://gentasks:secret@postgres:5432/genwork",
		"GENTASKS_HTTP_ADDR":    ":8080",
		"CORE_INTERNAL_URL":     "http://core:8484/",
		"GENWORK_MODULE_SECRET": "module-secret-0123456789abcdef",
		"GENWORK_PUBLIC_URL":    "http://localhost:8487",
	}
}

func getenvFrom(values map[string]string) platform.Getenv {
	return func(name string) string { return values[name] }
}

func TestLoadServeConfigAcceptsCompleteEnvironment(t *testing.T) {
	config, err := platform.LoadServeConfig(getenvFrom(validServeEnv()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.CoreInternalURL != "http://core:8484" || config.PublicURL != "http://localhost:8487" || config.HTTPAddr != ":8080" {
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
	cases := map[string]string{
		"CORE_INTERNAL_URL":     "core:8484",
		"GENWORK_PUBLIC_URL":    "ftp://localhost",
		"GENWORK_MODULE_SECRET": "short",
	}
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

func TestLoadDatabaseURLRequiresVariable(t *testing.T) {
	if _, err := platform.LoadDatabaseURL(getenvFrom(nil)); err == nil || !strings.Contains(err.Error(), "GENTASKS_DATABASE_URL") {
		t.Errorf("error = %v", err)
	}
	url, err := platform.LoadDatabaseURL(getenvFrom(validServeEnv()))
	if err != nil || url == "" {
		t.Errorf("url = %q, err = %v", url, err)
	}
}

func TestNewPublicTokenHas128RandomBits(t *testing.T) {
	first, second := platform.NewPublicToken(), platform.NewPublicToken()
	if len(first) != 22 || first == second || strings.ContainsAny(first, "+/=") {
		t.Errorf("tokens = %q, %q", first, second)
	}
}
