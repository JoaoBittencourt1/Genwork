package manifest_test

import (
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/command"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

func TestModuleManifestIsValid(t *testing.T) {
	parsed, err := manifest.ParseManifest(genmonitor.ManifestYAML, command.Version)
	if err != nil {
		t.Fatalf("parse module.yaml: %v", err)
	}
	if parsed.ID != "genmonitor" || parsed.APIPrefix != "/api/genmonitor/v1" || len(parsed.Requires) != 0 {
		t.Errorf("manifest = %+v", parsed)
	}
}

func TestPermissionConstantsMatchManifest(t *testing.T) {
	parsed, err := manifest.ParseManifest(genmonitor.ManifestYAML, command.Version)
	if err != nil {
		t.Fatalf("parse module.yaml: %v", err)
	}
	declared := make([]string, 0, len(parsed.Permissions))
	for _, permission := range parsed.Permissions {
		declared = append(declared, permission.Name)
	}
	if !slices.Equal(declared, access.Permissions) {
		t.Errorf("module.yaml declares %v, Go constants are %v", declared, access.Permissions)
	}
}
