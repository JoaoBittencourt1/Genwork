package modules

import "github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"

func moduleFixture(id, version string, requires ...manifest.Requirement) manifest.ModuleManifest {
	return manifest.ModuleManifest{ID: id, Version: version, Core: ">=1.0.0", APIPrefix: "/api/" + id + "/v1", Requires: requires}
}

func requirement(id, versionRange string) manifest.Requirement {
	return manifest.Requirement{ID: id, Version: versionRange}
}

// catalogFixture: reports requires gentasks; dashboards requires reports.
func catalogFixture() ModuleCatalog {
	return NewModuleCatalog(
		moduleFixture("gentasks", "0.1.0"),
		moduleFixture("reports", "1.0.0", requirement("gentasks", ">=0.1.0")),
		moduleFixture("dashboards", "1.0.0", requirement("reports", ">=1.0.0")),
		moduleFixture("wiki", "2.0.0"),
	)
}
