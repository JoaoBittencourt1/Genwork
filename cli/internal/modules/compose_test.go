package modules

import (
	"os"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeService struct {
	Profiles  []string       `yaml:"profiles"`
	DependsOn map[string]any `yaml:"depends_on"`
}

// The real compose file and catalog: every module the installer can install must be startable through its profile (design §11).
func TestComposeServicesMatchTheCatalog(t *testing.T) {
	catalog, warnings, err := LoadModuleCatalog("../../..")
	if err != nil || len(warnings) > 0 {
		t.Fatalf("catalog: %v %v", err, warnings)
	}
	data, err := os.ReadFile("../../../deploy/docker-compose.yml")
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	var compose struct {
		Services map[string]composeService `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose file: %v", err)
	}

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
