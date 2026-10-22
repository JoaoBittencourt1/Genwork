package modules

import (
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeService struct {
	Image     string         `yaml:"image"`
	Build     *composeBuild  `yaml:"build"`
	Profiles  []string       `yaml:"profiles"`
	DependsOn map[string]any `yaml:"depends_on"`
}

type composeBuild struct {
	Context    string            `yaml:"context"`
	Dockerfile string            `yaml:"dockerfile"`
	Args       map[string]string `yaml:"args"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

const (
	publishedImagePrefix = "${GENWORK_IMAGE_REGISTRY:?"
	publishedImageTag    = ":${GENWORK_VERSION:?"
)

func readComposeFile(t *testing.T, path string) composeFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose file: %v", err)
	}
	return compose
}

// The real compose file and catalog: every module the installer can install must be startable through its profile (design §11).
func TestComposeServicesMatchTheCatalog(t *testing.T) {
	catalog, warnings, err := LoadModuleCatalog("../../..", "0.0.0-dev")
	if err != nil || len(warnings) > 0 {
		t.Fatalf("catalog: %v %v", err, warnings)
	}
	compose := readComposeFile(t, "../../../deploy/docker-compose.yml")

	moduleServices := map[string]string{}
	for _, id := range catalog.IDs() {
		conventions := ConventionsFor(id)
		for _, name := range conventions.Services() {
			service, found := compose.Services[name]
			if !found {
				t.Errorf("module %s has no %q service", id, name)
				continue
			}
			if !slices.Equal(service.Profiles, []string{conventions.Profile}) {
				t.Errorf("service %q profiles = %v, want [%s]", name, service.Profiles, conventions.Profile)
			}
			moduleServices[name] = id
		}
	}
	for name, service := range compose.Services {
		for dependency := range service.DependsOn {
			owner, isModuleService := moduleServices[dependency]
			if isModuleService && moduleServices[name] != owner {
				t.Errorf("service %q depends on module service %q; it would fail whenever %s is not installed", name, dependency, owner)
			}
		}
	}
}

// A release installation never builds: every Genwork service runs the published image of the platform version (design §2).
func TestBaseComposeFileRunsPublishedImagesOnly(t *testing.T) {
	compose := readComposeFile(t, "../../../deploy/docker-compose.yml")

	for name, service := range compose.Services {
		if service.Build != nil {
			t.Errorf("service %q builds an image; builds belong in docker-compose.dev.yml", name)
		}
		if name == "postgres" {
			continue
		}
		if !strings.HasPrefix(service.Image, publishedImagePrefix) || !strings.Contains(service.Image, "/genwork-") ||
			!strings.Contains(service.Image, publishedImageTag) {
			t.Errorf("service %q image = %q, want ${GENWORK_IMAGE_REGISTRY:?…}/genwork-<name>:${GENWORK_VERSION:?…}", name, service.Image)
		}
	}
	if _, found := compose.Services["mailpit"]; found {
		t.Error("the mail catcher is a development service")
	}
}

func TestDevComposeFileBuildsEveryGenworkImageWithThePlatformVersion(t *testing.T) {
	base := readComposeFile(t, "../../../deploy/docker-compose.yml")
	dev := readComposeFile(t, "../../../deploy/docker-compose.dev.yml")

	for name := range base.Services {
		if name == "postgres" || name == "gateway" {
			continue
		}
		override, found := dev.Services[name]
		if !found || override.Build == nil {
			t.Errorf("service %q has no build in the dev override", name)
			continue
		}
		if override.Build.Args["VERSION"] != "${GENWORK_VERSION:?run compose through genwork}" {
			t.Errorf("service %q build args = %v", name, override.Build.Args)
		}
		if !strings.HasPrefix(override.Image, "genwork-") || !strings.HasSuffix(override.Image, ":dev") {
			t.Errorf("service %q dev image = %q; a dev build must never overwrite a release tag", name, override.Image)
		}
	}
	if gateway := dev.Services["gateway"]; gateway.Build != nil || !strings.HasPrefix(gateway.Image, "caddy:") {
		t.Errorf("the dev gateway runs plain Caddy, since Vite serves the web app: %+v", gateway)
	}
	if _, found := dev.Services["mailpit"]; !found {
		t.Error("the dev override runs the mail catcher")
	}
	if _, found := dev.Services["core"].DependsOn["mailpit"]; !found {
		t.Error("core waits for the mail catcher in development")
	}
}
