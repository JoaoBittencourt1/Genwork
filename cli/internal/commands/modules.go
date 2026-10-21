package commands

import (
	"errors"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/spf13/cobra"
)

var ErrNotInteractive = errors.New("`genwork modules` needs an interactive terminal to show the module checklist")

func newModulesCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "modules",
		Short: "Choose installed modules: checked modules are installed, unchecked ones are uninstalled",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !dependencies.IsInteractive() {
				return ErrNotInteractive
			}
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			setup, err := loadModuleSetup(command, dependencies, workspace)
			if err != nil {
				return err
			}
			return chooseModules(setup)
		},
	}
}

func chooseModules(setup *moduleSetup) error {
	checked, err := setup.dependencies.Selector.ChooseModules("Modules", setup.moduleOptions())
	if err != nil {
		return err
	}
	plan, err := modules.PlanSelection(setup.catalog, setup.config.Modules, checked)
	if err != nil {
		return err
	}
	output := setup.command.ErrOrStderr()
	if plan.IsEmpty() {
		fmt.Fprintln(output, "Nothing to change.")
		return nil
	}
	confirmed, err := setup.dependencies.Selector.ConfirmPlan(setup.summarize(plan), len(plan.Uninstall) > 0)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(output, "Nothing changed.")
		return nil
	}
	return setup.applyPlan(plan)
}

// chooseModulesOnFirstRun asks once per installation; without a terminal it records "no modules" and explains how to add them later.
func chooseModulesOnFirstRun(setup *moduleSetup) error {
	if setup.config.Chosen {
		return nil
	}
	output := setup.command.ErrOrStderr()
	if !setup.dependencies.IsInteractive() {
		fmt.Fprintln(output, "No modules installed. Run `genwork modules` in a terminal to choose them.")
		return setup.saveChoice(nil)
	}
	checked, err := setup.dependencies.Selector.ChooseModules("Choose the modules to install", setup.moduleOptions())
	if err != nil {
		return err
	}
	plan, err := modules.PlanSelection(setup.catalog, nil, checked)
	if err != nil {
		return err
	}
	if !plan.IsEmpty() {
		fmt.Fprint(output, setup.summarize(plan))
	}
	return setup.saveChoice(plan.Install)
}
