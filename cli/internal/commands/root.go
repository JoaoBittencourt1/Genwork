package commands

import (
	"os"

	"github.com/spf13/cobra"
)

type Dependencies struct {
	Runner        CommandRunner
	Prompter      Prompter
	Selector      ModuleSelector
	IsInteractive func() bool
	WorkingDir    func() (string, error)
	Getenv        func(name string) string
	// Executable is the running genwork binary, which `genwork update` replaces.
	Executable func() (string, error)
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		Runner:        ExecRunner{},
		Prompter:      NewTerminalPrompter(),
		Selector:      HuhModuleSelector{},
		IsInteractive: isInteractiveTerminal,
		WorkingDir:    os.Getwd,
		Getenv:        os.Getenv,
		Executable:    os.Executable,
	}
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "genwork",
		Short:         "Run and operate a Genwork instance",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(command *cobra.Command, _ []string) {
			warnWhenBinaryIsOutdated(command, dependencies)
		},
	}
	root.AddCommand(
		newDatabaseCommand(dependencies),
		newAPICommand(dependencies),
		newWebCommand(dependencies),
		newUpCommand(dependencies),
		newDownCommand(dependencies),
		newAuthCommand(dependencies),
		newModulesCommand(dependencies),
		newInstallCommand(dependencies),
		newUpdateCommand(dependencies),
		newUpdaterCommand(dependencies),
		newVersionCommand(),
	)
	return root
}

func locateWorkspace(dependencies Dependencies) (Workspace, error) {
	directory, err := dependencies.WorkingDir()
	if err != nil {
		return Workspace{}, err
	}
	return FindWorkspace(directory, dependencies.lookupEnv(genworkHomeVariable))
}

func (dependencies Dependencies) lookupEnv(name string) string {
	if dependencies.Getenv == nil {
		return ""
	}
	return dependencies.Getenv(name)
}
