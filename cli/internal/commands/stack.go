package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/spf13/cobra"
)

// ErrReported means the failing process already printed its own message to stderr.
var ErrReported = errors.New("command failed")

var ErrWebInReleaseInstallation = errors.New("the gateway serves the web app in a release installation; " +
	"`genwork web` runs the dev server of a repository checkout")

func newDatabaseCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "db",
		Short: "Start PostgreSQL",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			setup, err := loadModuleSetup(command, dependencies, workspace)
			if err != nil {
				return err
			}
			if err := setup.startDatabase(); err != nil {
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
		Short: "Start the gateway, core API, MCP server, updater, and modules with their database, migrations, worker, and mail catcher",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			if err := startStack(command, dependencies, workspace, "core", "core-worker", gatewayService, mcpService, updaterService); err != nil {
				return err
			}
			printStackEndpoints(command, workspace)
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
			if workspace.Kind() == WorkspaceRelease {
				return ErrWebInReleaseInstallation
			}
			return runWebDevServer(command, dependencies, workspace)
		},
	}
}

func newUpCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start database, API, and web app (in a repository checkout the web dev server runs in the foreground)",
		RunE: func(command *cobra.Command, _ []string) error {
			workspace, err := prepareWorkspace(command, dependencies)
			if err != nil {
				return err
			}
			if err := startStack(command, dependencies, workspace); err != nil {
				return err
			}
			printStackEndpoints(command, workspace)
			if workspace.Kind() == WorkspaceRelease {
				return nil
			}
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
			return runCompose(command, dependencies, workspace, "--profile", "*", "down")
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

// startStack starts the given services plus every installed module; with no names it starts the whole stack.
// On an installation that never chose its modules, it asks first.
func startStack(command *cobra.Command, dependencies Dependencies, workspace Workspace, services ...string) error {
	setup, err := loadModuleSetup(command, dependencies, workspace)
	if err != nil {
		return err
	}
	firstRun := !setup.config.Chosen
	if err := chooseModulesOnFirstRun(setup); err != nil {
		return err
	}
	if err := setup.startDatabase(); err != nil {
		return err
	}
	if err := setup.writeGatewayConfig(); err != nil {
		return err
	}
	if len(services) > 0 {
		for _, id := range setup.config.Modules {
			services = append(services, modules.ConventionsFor(id).Services()...)
		}
	}
	if err := setup.compose(append(setup.upArgs(), services...)...); err != nil {
		return err
	}
	if firstRun {
		for _, id := range setup.config.Modules {
			if err := setup.setModuleState(id, "enable"); err != nil {
				return err
			}
		}
	}
	return setup.restartGatewayIfRunning()
}

func runCompose(command *cobra.Command, dependencies Dependencies, workspace Workspace, args ...string) error {
	return runComposeCall(command, dependencies, workspace, composeCall{args: args})
}

// composeCall is one docker compose invocation; stdout defaults to the command's output.
type composeCall struct {
	args   []string
	stdin  io.Reader
	stdout io.Writer
	// version replaces VERSION as the image tag, e.g. the target of an update.
	version string
	// files replaces the workspace's compose files, e.g. a staged bundle's.
	files []string
}

// runComposeCall is how commands call compose; every image tag needs GENWORK_VERSION, which VERSION owns (design §2).
func runComposeCall(command *cobra.Command, dependencies Dependencies, workspace Workspace, call composeCall) error {
	if call.stdout == nil {
		call.stdout = command.OutOrStdout()
	}
	return runDockerCompose(command.Context(), dependencies, workspace, call, command.ErrOrStderr())
}

func runDockerCompose(ctx context.Context, dependencies Dependencies, workspace Workspace, call composeCall, stderr io.Writer) error {
	if call.version == "" {
		version, err := workspace.Version()
		if err != nil {
			return err
		}
		call.version = version
	}
	if call.files == nil {
		call.files = workspace.ComposeFiles()
	}
	err := dependencies.Runner.Run(ctx, RunRequest{
		Name:   "docker",
		Args:   append(append([]string{"compose"}, call.files...), call.args...),
		Dir:    workspace.Root,
		Env:    []string{"GENWORK_VERSION=" + call.version},
		Stdin:  call.stdin,
		Stdout: call.stdout,
		Stderr: stderr,
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
	fmt.Fprintln(command.ErrOrStderr(), "Installing web dependencies…")
	if err := run("install"); err != nil {
		return ErrReported
	}
	warnWhenProxySkipsGateway(command, workspace)
	fmt.Fprintf(command.ErrOrStderr(), "Web app: http://localhost:%s (Ctrl+C to stop; containers keep running, use `genwork down`)\n",
		workspace.WebEnvValue("DEV_SERVER_PORT"))
	if err := run("run", "dev"); err != nil && command.Context().Err() == nil {
		return ErrReported
	}
	return nil
}

// warnWhenProxySkipsGateway flags web/.env files written before the gateway existed: module APIs are only reachable through it.
func warnWhenProxySkipsGateway(command *cobra.Command, workspace Workspace) {
	gateway := "http://localhost:" + workspace.StackEnvValue("GATEWAY_PORT")
	target := workspace.WebEnvValue("DEV_API_PROXY_TARGET")
	if target == gateway {
		return
	}
	if target == "" {
		target = "(not set)"
	}
	banner := strings.Repeat("!", 78)
	fmt.Fprintf(command.ErrOrStderr(), "\n%s\n!! WARNING: web/.env DEV_API_PROXY_TARGET is %s, not the gateway (%s).\n"+
		"!! Module APIs (boards, tasks, ...) will answer 404 in the web app.\n"+
		"!! Fix: set DEV_API_PROXY_TARGET=%s in web/.env and restart.\n%s\n\n",
		banner, target, gateway, gateway, banner)
}

type endpoint int

const (
	endpointGateway endpoint = iota
	endpointWebApp
	endpointAPI
	endpointMail
	endpointDatabase
)

// printStackEndpoints lists what runs: a release installation has no mail catcher, and its gateway serves the web app.
func printStackEndpoints(command *cobra.Command, workspace Workspace) {
	if workspace.Kind() == WorkspaceRelease {
		printEndpoints(command, workspace, endpointWebApp, endpointAPI, endpointDatabase)
		return
	}
	printEndpoints(command, workspace, endpointGateway, endpointAPI, endpointMail, endpointDatabase)
}

func printEndpoints(command *cobra.Command, workspace Workspace, endpoints ...endpoint) {
	output := command.ErrOrStderr()
	for _, which := range endpoints {
		switch which {
		case endpointGateway:
			fmt.Fprintf(output, "Gateway:  http://localhost:%s\n", workspace.StackEnvValue("GATEWAY_PORT"))
		case endpointWebApp:
			fmt.Fprintf(output, "Web app:  http://localhost:%s\n", workspace.StackEnvValue("GATEWAY_PORT"))
		case endpointAPI:
			fmt.Fprintf(output, "Core API: http://localhost:%s\n", workspace.StackEnvValue("CORE_PORT"))
		case endpointMail:
			fmt.Fprintf(output, "Mailpit:  http://localhost:%s\n", workspace.StackEnvValue("MAILPIT_UI_PORT"))
		case endpointDatabase:
			fmt.Fprintf(output, "Postgres: localhost:%s\n", workspace.StackEnvValue("POSTGRES_PORT"))
		}
	}
}
