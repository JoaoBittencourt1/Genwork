package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the platform version of this build, injected with -ldflags -X (rule 50).
var Version = "0.0.0-dev"

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of this genwork binary",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(command.OutOrStdout(), Version)
			return err
		},
	}
}
