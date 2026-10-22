// Package modules holds the installer's module logic: catalog, installation choice, selection plan, and gateway routes.
package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

// ModuleCatalog is every module the workspace ships, read from modules/<name>/module.yaml (rule 51).
type ModuleCatalog struct {
	manifests map[string]manifest.ModuleManifest
}

// LoadModuleCatalog skips invalid manifests and reports each one as a warning, so one broken module never hides the others.
// Every module carries the workspace's platform version (rule 50).
func LoadModuleCatalog(workspaceRoot, platformVersion string) (ModuleCatalog, []string, error) {
	paths, err := filepath.Glob(filepath.Join(workspaceRoot, "modules", "*", "module.yaml"))
	if err != nil {
		return ModuleCatalog{}, nil, fmt.Errorf("find module manifests: %w", err)
	}
	catalog := ModuleCatalog{manifests: map[string]manifest.ModuleManifest{}}
	var warnings []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		parsed, err := manifest.ParseManifest(data, platformVersion)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		catalog.manifests[parsed.ID] = parsed
	}
	return catalog, warnings, nil
}

func NewModuleCatalog(manifests ...manifest.ModuleManifest) ModuleCatalog {
	catalog := ModuleCatalog{manifests: map[string]manifest.ModuleManifest{}}
	for _, moduleManifest := range manifests {
		catalog.manifests[moduleManifest.ID] = moduleManifest
	}
	return catalog
}

func (catalog ModuleCatalog) Find(id string) (manifest.ModuleManifest, bool) {
	moduleManifest, found := catalog.manifests[id]
	return moduleManifest, found
}

func (catalog ModuleCatalog) IDs() []string {
	ids := make([]string, 0, len(catalog.manifests))
	for id := range catalog.manifests {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
