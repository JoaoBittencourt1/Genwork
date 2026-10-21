package modules

import (
	"slices"
	"strings"
	"testing"
)

func TestPlanSelectionWithNothingChanged(t *testing.T) {
	plan, err := PlanSelection(catalogFixture(), []string{"gentasks"}, []string{"gentasks"})
	if err != nil || !plan.IsEmpty() {
		t.Errorf("plan = %+v, err = %v", plan, err)
	}
}

func TestPlanSelectionAddsRequiredModulesDependenciesFirst(t *testing.T) {
	plan, err := PlanSelection(catalogFixture(), nil, []string{"dashboards"})

	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !slices.Equal(plan.Install, []string{"gentasks", "reports", "dashboards"}) || len(plan.Uninstall) != 0 {
		t.Errorf("plan = %+v", plan)
	}
	if plan.RequiredBy["gentasks"] != "reports" || plan.RequiredBy["reports"] != "dashboards" || plan.RequiredBy["dashboards"] != "" {
		t.Errorf("required by = %v", plan.RequiredBy)
	}
}

func TestPlanSelectionSkipsAlreadyInstalledRequirements(t *testing.T) {
	plan, err := PlanSelection(catalogFixture(), []string{"gentasks"}, []string{"gentasks", "reports"})
	if err != nil || !slices.Equal(plan.Install, []string{"reports"}) || len(plan.RequiredBy) != 0 {
		t.Errorf("plan = %+v, err = %v", plan, err)
	}
}

func TestPlanSelectionUninstallsUncheckedModulesDependentsFirst(t *testing.T) {
	plan, err := PlanSelection(catalogFixture(), []string{"gentasks", "reports", "dashboards", "wiki"}, []string{"wiki"})

	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !slices.Equal(plan.Uninstall, []string{"dashboards", "reports", "gentasks"}) || len(plan.Install) != 0 {
		t.Errorf("plan = %+v", plan)
	}
}

func TestPlanSelectionRefusesToUninstallARequiredModule(t *testing.T) {
	_, err := PlanSelection(catalogFixture(), []string{"gentasks", "reports"}, []string{"reports"})
	if err == nil || !strings.Contains(err.Error(), `"gentasks"`) || !strings.Contains(err.Error(), `"reports" requires it`) {
		t.Errorf("error = %v", err)
	}
}

func TestPlanSelectionRefusesBrokenRequirements(t *testing.T) {
	cases := map[string]struct {
		catalog ModuleCatalog
		checked []string
		message string
	}{
		"unknown module": {catalogFixture(), []string{"nope"}, `"nope" is not in the catalog`},
		"missing requirement": {
			NewModuleCatalog(moduleFixture("reports", "1.0.0", requirement("gentasks", ">=0.1.0"))),
			[]string{"reports"}, `"reports" requires "gentasks", which is not in the catalog`,
		},
		"unsatisfied range": {
			NewModuleCatalog(moduleFixture("gentasks", "0.1.0"), moduleFixture("reports", "1.0.0", requirement("gentasks", ">=2.0.0"))),
			[]string{"reports"}, "but the catalog has 0.1.0",
		},
		"cycle": {
			NewModuleCatalog(moduleFixture("a", "1.0.0", requirement("b", ">=1.0.0")), moduleFixture("b", "1.0.0", requirement("a", ">=1.0.0"))),
			[]string{"a"}, "cycle",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := PlanSelection(testCase.catalog, nil, testCase.checked); err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Errorf("error = %v, want %q", err, testCase.message)
			}
		})
	}
}

func TestPlanSelectionHandlesModulesMissingFromTheCatalog(t *testing.T) {
	kept, err := PlanSelection(catalogFixture(), []string{"legacy"}, []string{"legacy"})
	if err != nil || !kept.IsEmpty() {
		t.Errorf("kept = %+v, err = %v", kept, err)
	}
	removed, err := PlanSelection(catalogFixture(), []string{"legacy"}, nil)
	if err != nil || !slices.Equal(removed.Uninstall, []string{"legacy"}) {
		t.Errorf("removed = %+v, err = %v", removed, err)
	}
}
