package platform_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
)

func validServeEnv() map[string]string {
	return map[string]string{
		"GENAGENTS_DATABASE_URL": "postgres://genagents:secret@postgres:5432/genwork",
		"GENAGENTS_HTTP_ADDR":    ":8080",
		"CORE_INTERNAL_URL":      "http://core:8484/",
		"GATEWAY_INTERNAL_URL":   "http://gateway:8487/",
		"GENWORK_MODULE_SECRET":  "module-secret-0123456789abcdef",
		"GENWORK_PUBLIC_URL":     "http://localhost:8487",
		// 32 zero bytes as unpadded base64url.
		"GENAGENTS_ENCRYPTION_KEY": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
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
	if config.CoreInternalURL != "http://core:8484" || config.GatewayInternalURL != "http://gateway:8487" ||
		config.PublicURL != "http://localhost:8487" || config.HTTPAddr != ":8080" || len(config.EncryptionKey) != 32 {
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

func TestLoadServeConfigRejectsInvalidValuesWithoutEchoingTheKey(t *testing.T) {
	cases := map[string]string{
		"CORE_INTERNAL_URL":        "core:8484",
		"GATEWAY_INTERNAL_URL":     "gateway",
		"GENWORK_PUBLIC_URL":       "ftp://localhost",
		"GENWORK_MODULE_SECRET":    "short",
		"GENAGENTS_ENCRYPTION_KEY": "c2hvcnQ",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			env := validServeEnv()
			env[name] = value
			_, err := platform.LoadServeConfig(getenvFrom(env))
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("error = %v, want mention of %s", err, name)
			}
			if name == "GENAGENTS_ENCRYPTION_KEY" && strings.Contains(err.Error(), value) {
				t.Error("the error echoes the encryption key")
			}
		})
	}
}

func TestLoadDatabaseURLRequiresTheVariable(t *testing.T) {
	if _, err := platform.LoadDatabaseURL(getenvFrom(map[string]string{})); err == nil {
		t.Error("missing GENAGENTS_DATABASE_URL was accepted")
	}
	url, err := platform.LoadDatabaseURL(getenvFrom(map[string]string{"GENAGENTS_DATABASE_URL": "postgres://x"}))
	if err != nil || url != "postgres://x" {
		t.Errorf("url = %q, err = %v", url, err)
	}
}
