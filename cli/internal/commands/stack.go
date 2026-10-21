package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// ErrReported means the failing process already printed its own message to stderr.
var ErrReported = errors.New("command failed")

func newDatabaseCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "db",
		Short: "Start PostgreSQL",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			if err := startServices(command, dependencies, workspace, "postgres"); err != nil {
				return err
			}
			printEndpoints(command, workspace, endpointDatabase)
			return nil
		},
	}
}

func newAPICommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Start the core API with its database, migrations, worker, and mail catcher",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			if err := startServices(command, dependencies, workspace, "core", "core-worker"); err != nil {
				return err
			}
			printEndpoints(command, workspace, endpointAPI, endpointMail, endpointDatabase)
			return nil
		},
	}
}

func newWebCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "web",
		Short: "Run the web app dev server in the foreground",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			return runWebDevServer(command, dependencies, workspace)
		},
	}
}

func newUpCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start database, API, and web app (the web app runs in the foreground)",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			if err := startServices(command, dependencies, workspace); err != nil {
				return err
			}
			printEndpoints(command, workspace, endpointAPI, endpointMail, endpointDatabase)
			return runWebDevServer(command, dependencies, workspace)
		},
	}
}

func newDownCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the database, API, and mail catcher (data is kept)",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := locateWorkspace(dependencies)
			if err != nil {
				return err
			}
			return runCompose(command, dependencies, workspace, "down")
		},
	}
}

func prepareWorkspace(command *cobra.Command, dependencies Dependencies) (Workspace, error) {
	workspace, err := locateWorkspace(dependencies)
	if err != nil {
		return Workspace{}, err
	}
	return workspace, workspace.EnsureEnvFiles(command.ErrOrStderr())
}

// startServices with no names starts every service in the stack.
func startServices(command *cobra.Command, dependencies Dependencies, workspace Workspace, services ...string) error {
	return runCompose(command, dependencies, workspace, append([]string{"up", "-d", "--build"}, services...)...)
}

func runCompose(command *cobra.Command, dependencies Dependencies, workspace Workspace, args ...string) error {
	err := dependencies.Runner.Run(command.Context(), RunRequest{
		Name:   "docker",
		Args:   append([]string{"compose", "-f", workspace.ComposeFile()}, args...),
		Dir:    workspace.Root,
		Stdout: command.OutOrStdout(),
		Stderr: command.ErrOrStderr(),
	})
	if err != nil {
		return ErrReported
	}
	return nil
}

func runWebDevServer(command *cobra.Command, dependencies Dependencies, workspace Workspace) error {
	run := func(args ...string) error {
		return dependencies.Runner.Run(command.Context(), RunRequest{
			Name: "npm", Args: args, Dir: workspace.WebDir(),
			Stdin: os.Stdin, Stdout: command.OutOrStdout(), Stderr: command.ErrOrStderr(),
		})
	}
	if _, err := os.Stat(filepath.Join(workspace.WebDir(), "node_modules")); err != nil {
		fmt.Fprintln(command.ErrOrStderr(), "Installing web dependencies…")
		if err := run("install"); err != nil {
			return ErrReported
		}
	}
	fmt.Fprintf(command.ErrOrStderr(), "Web app: http://localhost:%s (Ctrl+C to stop; containers keep running, use `genwork down`)\n",
		workspace.WebEnvValue("DEV_SERVER_PORT"))
	if err := run("run", "dev"); err != nil && command.Context().Err() == nil {
		return ErrReported
	}
	return nil
}

type endpoint int

const (
	endpointAPI endpoint = iota
	endpointMail
	endpointDatabase
)

func printEndpoints(command *cobra.Command, workspace Workspace, endpoints ...endpoint) {
	output := command.ErrOrStderr()
	for _, which := range endpoints {
		switch which {
		case endpointAPI:
			fmt.Fprintf(output, "Core API: http://localhost:%s\n", workspace.StackEnvValue("CORE_PORT"))
		case endpointMail:
			fmt.Fprintf(output, "Mailpit:  http://localhost:%s\n", workspace.StackEnvValue("MAILPIT_UI_PORT"))
		case endpointDatabase:
			fmt.Fprintf(output, "Postgres: localhost:%s\n", workspace.StackEnvValue("POSTGRES_PORT"))
		}
	}
}
