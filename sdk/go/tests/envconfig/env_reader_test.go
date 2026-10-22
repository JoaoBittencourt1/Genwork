package envconfig_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/envconfig"
)

func getenvFrom(values map[string]string) envconfig.Getenv {
	return func(name string) string { return values[name] }
}

func TestEnvReaderReturnsTrimmedValuesWithoutError(t *testing.T) {
	reader := envconfig.NewEnvReader(getenvFrom(map[string]string{
		"HTTP_ADDR":     " :8080 ",
		"CORE_URL":      "http://core:8484/",
		"MODULE_SECRET": "module-secret-0123456789abcdef",
	}))

	addr, coreURL, secret := reader.Required("HTTP_ADDR"), reader.RequiredURL("CORE_URL"), reader.RequiredModuleSecret("MODULE_SECRET")

	if err := reader.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != ":8080" || coreURL != "http://core:8484" || secret != "module-secret-0123456789abcdef" {
		t.Errorf("values = %q, %q, %q", addr, coreURL, secret)
	}
}

func TestEnvReaderReportsEveryMissingVariable(t *testing.T) {
	reader := envconfig.NewEnvReader(getenvFrom(nil))
	reader.Required("FIRST")
	reader.RequiredURL("SECOND")
	reader.RequiredModuleSecret("THIRD")

	err := reader.Err()

	if err == nil || !strings.Contains(err.Error(), "missing required environment variables: FIRST, SECOND, THIRD") {
		t.Errorf("error = %v", err)
	}
}

func TestEnvReaderRejectsURLsThatAreNotAbsoluteHTTP(t *testing.T) {
	for _, value := range []string{"core:8484", "ftp://core", "http://"} {
		reader := envconfig.NewEnvReader(getenvFrom(map[string]string{"CORE_URL": value}))
		reader.RequiredURL("CORE_URL")
		if err := reader.Err(); err == nil || !strings.Contains(err.Error(), "invalid environment variables: CORE_URL") {
			t.Errorf("value %q: error = %v", value, err)
		}
	}
}

func TestEnvReaderRejectsShortModuleSecretsWithoutPrintingThem(t *testing.T) {
	reader := envconfig.NewEnvReader(getenvFrom(map[string]string{"MODULE_SECRET": "short-secret"}))
	reader.RequiredModuleSecret("MODULE_SECRET")

	err := reader.Err()

	if err == nil || !strings.Contains(err.Error(), "MODULE_SECRET") || strings.Contains(err.Error(), "short-secret") {
		t.Errorf("error = %v", err)
	}
}

func TestEnvReaderJoinsMissingAndInvalidProblems(t *testing.T) {
	reader := envconfig.NewEnvReader(getenvFrom(nil))
	reader.Required("ADDR")
	reader.MarkInvalid("KEY (too short)")

	err := reader.Err()

	if err == nil || err.Error() != "missing required environment variables: ADDR; invalid environment variables: KEY (too short)" {
		t.Errorf("error = %v", err)
	}
}
