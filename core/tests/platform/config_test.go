package platform_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"strings"
	"testing"
)

func validServeEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":           "postgres://genwork:secret@postgres:5432/genwork",
		"CORE_HTTP_ADDR":         ":8484",
		"GENWORK_PUBLIC_URL":     "http://localhost:8485/",
		"GENWORK_INSTANCE_NAME":  "Genwork Test",
		"GENWORK_DEFAULT_LOCALE": "pt-BR",
		"JWT_SIGNING_KEY":        "c2VlZA==",
		"SMTP_HOST":              "mailpit",
		"SMTP_PORT":              "1025",
		"SMTP_FROM":              "Genwork <no-reply@genwork.local>",
		"SMTP_TLS":               "none",
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
	if config.PublicURL != "http://localhost:8485" {
		t.Errorf("public URL should drop the trailing slash, got %q", config.PublicURL)
	}
	if config.SMTP.Port != 1025 {
		t.Errorf("SMTP port = %d, want 1025", config.SMTP.Port)
	}
}

func TestLoadServeConfigListsEveryMissingVariable(t *testing.T) {
	env := validServeEnv()
	delete(env, "GENWORK_INSTANCE_NAME")
	delete(env, "JWT_SIGNING_KEY")

	_, err := platform.LoadServeConfig(getenvFrom(env))
	if err == nil {
		t.Fatal("expected an error for missing variables")
	}
	for _, name := range []string{"GENWORK_INSTANCE_NAME", "JWT_SIGNING_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestLoadServeConfigTreatsBlankAsMissing(t *testing.T) {
	env := validServeEnv()
	env["GENWORK_INSTANCE_NAME"] = "   "
	if _, err := platform.LoadServeConfig(getenvFrom(env)); err == nil || !strings.Contains(err.Error(), "GENWORK_INSTANCE_NAME") {
		t.Fatalf("expected blank instance name to be rejected, got %v", err)
	}
}

func TestLoadServeConfigRejectsInvalidValues(t *testing.T) {
	cases := map[string]string{
		"GENWORK_PUBLIC_URL":     "localhost:8485",
		"GENWORK_DEFAULT_LOCALE": "fr",
		"SMTP_PORT":              "abc",
		"SMTP_TLS":               "maybe",
		"TRUSTED_PROXY_CIDRS":    "10.0.0.0/8,not-a-cidr",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			env := validServeEnv()
			env[name] = value
			if _, err := platform.LoadServeConfig(getenvFrom(env)); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("expected %s=%q to be rejected, got %v", name, value, err)
			}
		})
	}
}

func TestLoadServeConfigRequiresSMTPCredentialsTogether(t *testing.T) {
	env := validServeEnv()
	env["SMTP_USERNAME"] = "user"
	if _, err := platform.LoadServeConfig(getenvFrom(env)); err == nil || !strings.Contains(err.Error(), "SMTP_USERNAME") {
		t.Fatalf("expected username without password to be rejected, got %v", err)
	}
	env["SMTP_PASSWORD"] = "secret"
	if _, err := platform.LoadServeConfig(getenvFrom(env)); err != nil {
		t.Fatalf("expected username with password to be accepted, got %v", err)
	}
}

func TestLoadDatabaseURLRequiresVariable(t *testing.T) {
	if _, err := platform.LoadDatabaseURL(getenvFrom(map[string]string{})); err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}
}
