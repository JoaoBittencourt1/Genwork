package commands

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/spf13/cobra"
)

const (
	gatewayService = "gateway"
	mcpService     = "mcp"
)

// moduleSetup applies the installation's module choice to the stack: profiles, databases, gateway routes, and core.
type moduleSetup struct {
	command      *cobra.Command
	dependencies Dependencies
	workspace    Workspace
	catalog      modules.ModuleCatalog
	config       modules.InstallationConfig
}

func loadModuleSetup(command *cobra.Command, dependencies Dependencies, workspace Workspace) (*moduleSetup, error) {
	version, err := workspace.Version()
	if err != nil {
		return nil, err
	}
	catalog, warnings, err := modules.LoadModuleCatalog(workspace.Root, version)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintf(command.ErrOrStderr(), "Warning: skipping invalid module manifest %s\n", warning)
	}
	config, err := modules.ReadInstallationConfig(workspace.InstallationFile())
	if err != nil {
		return nil, err
	}
	return &moduleSetup{command: command, dependencies: dependencies, workspace: workspace, catalog: catalog, config: config}, nil
}

// compose runs docker compose with the profile of every installed module, so their services can be addressed.
func (setup *moduleSetup) compose(args ...string) error {
	var profiles []string
	for _, id := range setup.config.Modules {
		profiles = append(profiles, "--profile", modules.ConventionsFor(id).Profile)
	}
	return runCompose(setup.command, setup.dependencies, setup.workspace, append(profiles, args...)...)
}

// upArgs starts services; only a repository checkout builds its images, a release installation runs the published ones.
func (setup *moduleSetup) upArgs() []string {
	if setup.workspace.Kind() == WorkspaceRepository {
		return []string{"up", "-d", "--build"}
	}
	return []string{"up", "-d"}
}

// startDatabase waits for Postgres and provisions every installed module before any module migration runs.
func (setup *moduleSetup) startDatabase() error {
	if err := setup.workspace.EnsureModuleSecrets(setup.command.ErrOrStderr(), setup.config.Modules); err != nil {
		return err
	}
	if err := runCompose(setup.command, setup.dependencies, setup.workspace, "up", "-d", "--wait", "postgres"); err != nil {
		return err
	}
	for _, id := range setup.config.Modules {
		if err := provisionModuleDatabase(setup.command, setup.dependencies, setup.workspace, id); err != nil {
			return err
		}
	}
	return nil
}

func (setup *moduleSetup) writeGatewayConfig() error {
	return writeGatewayConfig(setup.workspace, setup.catalog, setup.config)
}

// writeGatewayConfig rewrites the file in place: the gateway bind-mounts it, and a rename would leave the container on the old copy.
func writeGatewayConfig(workspace Workspace, catalog modules.ModuleCatalog, config modules.InstallationConfig) error {
	var installed []manifest.ModuleManifest
	for _, id := range config.Modules {
		if moduleManifest, found := catalog.Find(id); found {
			installed = append(installed, moduleManifest)
		}
	}
	rendered, err := modules.RenderCaddyfile(installed)
	if err != nil {
		return err
	}
	if err := os.WriteFile(workspace.GatewayConfigFile(), rendered, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", workspace.GatewayConfigFile(), err)
	}
	return nil
}

func (setup *moduleSetup) restartGatewayIfRunning() error {
	running, err := runningServices(setup.command, setup.dependencies, setup.workspace)
	if err != nil || !slices.Contains(running, gatewayService) {
		return nil
	}
	return runCompose(setup.command, setup.dependencies, setup.workspace, "restart", gatewayService)
}

func (setup *moduleSetup) saveChoice(installed []string) error {
	if err := modules.WriteInstallationConfig(setup.workspace.InstallationFile(), installed); err != nil {
		return err
	}
	config, err := modules.ReadInstallationConfig(setup.workspace.InstallationFile())
	if err != nil {
		return err
	}
	setup.config = config
	return nil
}

// moduleOptions lists every catalog module, plus installed ones that left the catalog so they can still be unchecked.
func (setup *moduleSetup) moduleOptions() []ModuleOption {
	ids := setup.catalog.IDs()
	for _, id := range setup.config.Modules {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	options := make([]ModuleOption, 0, len(ids))
	for _, id := range ids {
		options = append(options, ModuleOption{ID: id, Label: setup.describeModule(id), Checked: setup.config.IsInstalled(id)})
	}
	return options
}

func (setup *moduleSetup) describeModule(id string) string {
	moduleManifest, found := setup.catalog.Find(id)
	if !found {
		return id + "  (missing from catalog)"
	}
	label := id
	if len(moduleManifest.Requires) > 0 {
		required := make([]string, 0, len(moduleManifest.Requires))
		for _, requirement := range moduleManifest.Requires {
			required = append(required, requirement.ID)
		}
		label += "  requires: " + strings.Join(required, ", ")
	}
	return label
}

func (setup *moduleSetup) summarize(plan modules.SelectionPlan) string {
	var summary strings.Builder
	if len(plan.Install) > 0 {
		summary.WriteString("Install:\n")
		for _, id := range plan.Install {
			fmt.Fprintf(&summary, "  + %s", setup.describeModule(id))
			if requiredBy, found := plan.RequiredBy[id]; found {
				fmt.Fprintf(&summary, "  (required by %s)", requiredBy)
			}
			summary.WriteString("\n")
		}
	}
	if len(plan.Uninstall) > 0 {
		summary.WriteString("Uninstall — the data below is permanently deleted:\n")
		for _, id := range plan.Uninstall {
			fmt.Fprintf(&summary, "  - %s: database schema %q and everything in it\n", id, modules.ConventionsFor(id).DatabaseRole)
		}
	}
	return summary.String()
}

// applyPlan uninstalls before installing, so a module and its replacement never compete for the same routes.
func (setup *moduleSetup) applyPlan(plan modules.SelectionPlan) error {
	for _, id := range plan.Uninstall {
		if err := setup.uninstallModule(id); err != nil {
			return err
		}
	}
	if len(plan.Install) > 0 {
		if err := setup.installModules(plan.Install); err != nil {
			return err
		}
	}
	if err := setup.writeGatewayConfig(); err != nil {
		return err
	}
	return setup.restartGatewayIfRunning()
}

// installModules records the choice first, so an interrupted install is completed by the next run (design §1).
func (setup *moduleSetup) installModules(ids []string) error {
	if err := setup.saveChoice(append(slices.Clone(setup.config.Modules), ids...)); err != nil {
		return err
	}
	if err := setup.startDatabase(); err != nil {
		return err
	}
	if err := setup.writeGatewayConfig(); err != nil {
		return err
	}
	var services []string
	for _, id := range ids {
		services = append(services, modules.ConventionsFor(id).Services()...)
	}
	if err := setup.compose(append(setup.upArgs(), services...)...); err != nil {
		return err
	}
	for _, id := range ids {
		if err := setup.setModuleState(id, "enable"); err != nil {
			return err
		}
		fmt.Fprintf(setup.command.ErrOrStderr(), "Installed %s\n", id)
	}
	return nil
}

// uninstallModule removes the module from the choice last, so it stays checked until every step succeeded.
func (setup *moduleSetup) uninstallModule(id string) error {
	conventions := modules.ConventionsFor(id)
	if err := setup.compose(append([]string{"rm", "-s", "-f"}, conventions.Services()...)...); err != nil {
		return err
	}
	if err := setup.setModuleState(id, "disable"); err != nil {
		return err
	}
	if err := runCompose(setup.command, setup.dependencies, setup.workspace, "up", "-d", "--wait", "postgres"); err != nil {
		return err
	}
	if err := deprovisionModuleDatabase(setup.command, setup.dependencies, setup.workspace, id); err != nil {
		return err
	}
	if err := setup.workspace.ClearModuleSecrets(id); err != nil {
		return err
	}
	remaining := slices.DeleteFunc(slices.Clone(setup.config.Modules), func(installed string) bool { return installed == id })
	if err := setup.saveChoice(remaining); err != nil {
		return err
	}
	fmt.Fprintf(setup.command.ErrOrStderr(), "Uninstalled %s\n", id)
	return nil
}

// setModuleState publishes or hides a module in core's capabilities (rules 56, 58); core is started if it is not running.
func (setup *moduleSetup) setModuleState(id, action string) error {
	if err := runCompose(setup.command, setup.dependencies, setup.workspace, "up", "-d", coreService); err != nil {
		return err
	}
	return runCompose(setup.command, setup.dependencies, setup.workspace, "exec", "-T", coreService, "/core", "module", action, id)
}

func runningServices(command *cobra.Command, dependencies Dependencies, workspace Workspace) ([]string, error) {
	var services bytes.Buffer
	call := composeCall{args: []string{"ps", "--status", "running", "--services"}, stdout: &services}
	if err := runComposeCall(command, dependencies, workspace, call); err != nil {
		return nil, err
	}
	return strings.Fields(services.String()), nil
}
