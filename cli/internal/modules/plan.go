package modules

import (
	"fmt"
	"slices"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/Masterminds/semver/v3"
)

// SelectionPlan is what a confirmed checklist changes: modules to install (dependencies first) and to uninstall (dependents first).
type SelectionPlan struct {
	Install   []string
	Uninstall []string
	// RequiredBy explains modules the user did not check but that are installed because a checked module requires them.
	RequiredBy map[string]string
}

func (plan SelectionPlan) IsEmpty() bool {
	return len(plan.Install) == 0 && len(plan.Uninstall) == 0
}

type selectionResolver struct {
	catalog    ModuleCatalog
	checked    []string
	visiting   map[string]bool
	resolved   map[string]bool
	order      []string
	requiredBy map[string]string
}

// PlanSelection turns the checked boxes into a plan, or refuses it before anything changes (rule 55).
func PlanSelection(catalog ModuleCatalog, installed, checked []string) (SelectionPlan, error) {
	resolver := &selectionResolver{
		catalog: catalog, checked: checked,
		visiting: map[string]bool{}, resolved: map[string]bool{}, requiredBy: map[string]string{},
	}
	sortedChecked := slices.Clone(checked)
	slices.Sort(sortedChecked)
	for _, id := range sortedChecked {
		if _, found := catalog.Find(id); !found {
			if slices.Contains(installed, id) {
				resolver.keep(id)
				continue
			}
			return SelectionPlan{}, fmt.Errorf("module %q is not in the catalog", id)
		}
		if err := resolver.visit(id); err != nil {
			return SelectionPlan{}, err
		}
	}
	for required, requiredBy := range resolver.requiredBy {
		if slices.Contains(installed, required) && !slices.Contains(checked, required) {
			return SelectionPlan{}, fmt.Errorf("cannot uninstall %q: %q requires it", required, requiredBy)
		}
	}

	plan := SelectionPlan{RequiredBy: map[string]string{}}
	for _, id := range resolver.order {
		if !slices.Contains(installed, id) {
			plan.Install = append(plan.Install, id)
			if requiredBy, found := resolver.requiredBy[id]; found {
				plan.RequiredBy[id] = requiredBy
			}
		}
	}
	var removed []string
	for _, id := range installed {
		if !resolver.resolved[id] {
			removed = append(removed, id)
		}
	}
	plan.Uninstall = dependentsFirst(catalog, removed)
	return plan, nil
}

func (resolver *selectionResolver) keep(id string) {
	if !resolver.resolved[id] {
		resolver.resolved[id] = true
		resolver.order = append(resolver.order, id)
	}
}

// visit walks requirements depth-first, so order lists every module after the modules it requires.
func (resolver *selectionResolver) visit(id string) error {
	if resolver.resolved[id] {
		return nil
	}
	if resolver.visiting[id] {
		return fmt.Errorf("modules require each other in a cycle through %q", id)
	}
	resolver.visiting[id] = true
	moduleManifest, _ := resolver.catalog.Find(id)
	for _, requirement := range moduleManifest.Requires {
		required, found := resolver.catalog.Find(requirement.ID)
		if !found {
			return fmt.Errorf("%q requires %q, which is not in the catalog", id, requirement.ID)
		}
		if !satisfies(requirement.Version, required.Version) {
			return fmt.Errorf("%q requires %q %s, but the catalog has %s", id, requirement.ID, requirement.Version, required.Version)
		}
		if _, alreadyExplained := resolver.requiredBy[requirement.ID]; !alreadyExplained && !slices.Contains(resolver.checked, requirement.ID) {
			resolver.requiredBy[requirement.ID] = id
		}
		if err := resolver.visit(requirement.ID); err != nil {
			return err
		}
	}
	resolver.visiting[id] = false
	resolver.keep(id)
	return nil
}

// dependentsFirst orders modules to uninstall so that no module is removed while another removed one still requires it.
func dependentsFirst(catalog ModuleCatalog, removed []string) []string {
	sorted := slices.Clone(removed)
	slices.Sort(sorted)
	var ordered []string
	placed := map[string]bool{}
	var place func(id string)
	place = func(id string) {
		if placed[id] {
			return
		}
		placed[id] = true
		for _, other := range sorted {
			if requires(catalog, other, id) {
				place(other)
			}
		}
		ordered = append(ordered, id)
	}
	for _, id := range sorted {
		place(id)
	}
	return ordered
}

func requires(catalog ModuleCatalog, id, required string) bool {
	moduleManifest, found := catalog.Find(id)
	if !found {
		return false
	}
	return slices.ContainsFunc(moduleManifest.Requires, func(requirement manifest.Requirement) bool { return requirement.ID == required })
}

func satisfies(versionRange, version string) bool {
	constraint, rangeErr := semver.NewConstraint(versionRange)
	parsed, versionErr := semver.NewVersion(version)
	return rangeErr == nil && versionErr == nil && constraint.Check(parsed)
}
