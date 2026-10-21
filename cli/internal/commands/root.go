package commands

import (
	"os"

	"github.com/spf13/cobra"
)

type Dependencies struct {
	Runner     CommandRunner
	Prompter   Prompter
	WorkingDir func() (string, error)
}

func DefaultDependencies() Dependencies {
	return Dependencies{Runner: ExecRunner{}, Prompter: NewTerminalPrompter(), WorkingDir: os.Getwd}
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "genwork",
		Short:         "Run and operate a Genwork instance",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newDatabaseCommand(dependencies),
		newAPICommand(dependencies),
		newWebCommand(dependencies),
		newUpCommand(dependencies),
		newDownCommand(dependencies),
		newAuthCommand(dependencies),
	)
	return root
}

func locateWorkspace(dependencies Dependencies) (Workspace, error) {
	directory, err := dependencies.WorkingDir()
	if err != nil {
		return Workspace{}, err
	}
	return FindWorkspace(directory)
}
