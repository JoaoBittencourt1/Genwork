package modules

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadModuleCatalogSkipsInvalidManifests(t *testing.T) {
	catalog, warnings, err := LoadModuleCatalog("testdata/workspace", "0.5.0")

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ids := catalog.IDs(); !slices.Equal(ids, []string{"gentasks", "reports"}) {
		t.Errorf("ids = %v", ids)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], filepath.Join("broken", "module.yaml")) {
		t.Errorf("warnings = %v", warnings)
	}
	reports, found := catalog.Find("reports")
	if !found || reports.Requires[0].ID != "gentasks" || reports.Version != "0.5.0" {
		t.Errorf("reports = %+v", reports)
	}
}

func TestLoadModuleCatalogOfAnEmptyWorkspace(t *testing.T) {
	catalog, warnings, err := LoadModuleCatalog(t.TempDir(), "0.5.0")
	if err != nil || len(catalog.IDs()) != 0 || len(warnings) != 0 {
		t.Errorf("catalog = %v, warnings = %v, err = %v", catalog.IDs(), warnings, err)
	}
}

func TestInstallationConfigTellsNeverChosenFromChoseNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "genwork.yaml")

	never, err := ReadInstallationConfig(path)
	if err != nil || never.Chosen {
		t.Fatalf("missing file = %+v, %v", never, err)
	}
	if err := WriteInstallationConfig(path, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	nothing, err := ReadInstallationConfig(path)
	if err != nil || !nothing.Chosen || len(nothing.Modules) != 0 {
		t.Errorf("empty choice = %+v, %v", nothing, err)
	}
}

func TestInstallationConfigRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "genwork.yaml")

	if err := WriteInstallationConfig(path, []string{"reports", "gentasks", "gentasks"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	config, err := ReadInstallationConfig(path)

	if err != nil || !slices.Equal(config.Modules, []string{"gentasks", "reports"}) || !config.IsInstalled("reports") || config.IsInstalled("wiki") {
		t.Errorf("config = %+v, %v", config, err)
	}
	content, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(content), "# Written by genwork") {
		t.Errorf("content = %s", content)
	}
}

func TestInstallationConfigRejectsMalformedFiles(t *testing.T) {
	for _, content := range []string{"modules: [unclosed", "plugins: [gentasks]\n"} {
		path := filepath.Join(t.TempDir(), "genwork.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadInstallationConfig(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%q: error = %v", content, err)
		}
	}
}

func TestWriteInstallationConfigFailsOnMissingDirectory(t *testing.T) {
	if err := WriteInstallationConfig(filepath.Join(t.TempDir(), "missing", "genwork.yaml"), nil); err == nil {
		t.Error("expected an error")
	}
}
