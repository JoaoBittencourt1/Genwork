package platform_test

import (
	"errors"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

func getenvWith(values map[string]string) platform.Getenv {
	return func(name string) string { return values[name] }
}

func TestLoadConfigTrimsTheTrailingSlash(t *testing.T) {
	config, err := platform.LoadConfig(getenvWith(map[string]string{"GENWORK_URL": " https://genwork.example.test/ "}))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.InstanceURL != "https://genwork.example.test" {
		t.Fatalf("InstanceURL = %q", config.InstanceURL)
	}
}

func TestLoadConfigFailsWithoutTheInstanceURL(t *testing.T) {
	_, err := platform.LoadConfig(getenvWith(nil))
	if !errors.Is(err, platform.ErrMissingInstanceURL) {
		t.Fatalf("err = %v, want ErrMissingInstanceURL", err)
	}
}

func TestLoadConfigRejectsAURLThatIsNotAbsoluteHTTP(t *testing.T) {
	for _, value := range []string{"genwork.example.test", "ftp://genwork.example.test", "http://"} {
		_, err := platform.LoadConfig(getenvWith(map[string]string{"GENWORK_URL": value}))
		if !errors.Is(err, platform.ErrInvalidInstanceURL) {
			t.Fatalf("%q: err = %v, want ErrInvalidInstanceURL", value, err)
		}
	}
}
