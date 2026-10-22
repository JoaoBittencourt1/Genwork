package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/updates"
)

const (
	updaterService      = "updater"
	databaseService     = "postgres"
	stackStartupTimeout = "300"
)

// workspaceInstallation runs an update's Docker steps on a workspace, for the host CLI and for the updater container.
type workspaceInstallation struct {
	dependencies Dependencies
	workspace    Workspace
	stdout       io.Writer
	stderr       io.Writer
	// gatewayURL is where ReportedVersion asks: GATEWAY_INTERNAL_URL in the updater, the published port on the host.
	gatewayURL string
	httpClient *http.Client
	// insideUpdater replaces the updater from a one-off container, since a container cannot recreate itself.
	insideUpdater bool
}

func newWorkspaceInstallation(dependencies Dependencies, workspace Workspace, stdout, stderr io.Writer, gatewayURL string) *workspaceInstallation {
	return &workspaceInstallation{
		dependencies: dependencies, workspace: workspace, stdout: stdout, stderr: stderr,
		gatewayURL: strings.TrimRight(gatewayURL, "/"), httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

var _ updates.Installation = (*workspaceInstallation)(nil)

func (installation *workspaceInstallation) Root() string { return installation.workspace.Root }

func (installation *workspaceInstallation) IsRepositoryCheckout() bool {
	return installation.workspace.Kind() == WorkspaceRepository
}

// PullImages reads the staged compose file with this installation's .env plus the keys the new example adds, so a
// variable the new release introduces cannot fail the pull.
func (installation *workspaceInstallation) PullImages(ctx context.Context, version, stagedRoot string) error {
	current, err := os.ReadFile(installation.workspace.StackEnvFile())
	if err != nil {
		return fmt.Errorf("read %s: %w", installation.workspace.StackEnvFile(), err)
	}
	merged, _ := appendKeysMissingFrom(string(current), filepath.Join(stagedRoot, "deploy", ".env.example"))
	pullEnv := filepath.Join(stagedRoot, "pull.env")
	if err := os.WriteFile(pullEnv, []byte(merged), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", pullEnv, err)
	}
	files := []string{
		"--project-directory", filepath.Join(installation.workspace.Root, "deploy"),
		"--env-file", pullEnv,
		"-f", filepath.Join(stagedRoot, "deploy", "docker-compose.yml"),
	}
	return installation.composeWithProfiles(ctx, composeCall{version: version, files: files, args: []string{"pull"}})
}

func (installation *workspaceInstallation) DumpDatabase(ctx context.Context, destination io.Writer) error {
	return installation.compose(ctx, composeCall{
		args:   append([]string{"exec", "-T", databaseService, "pg_dump", "-Fc"}, installation.databaseArgs()...),
		stdout: destination,
	})
}

func (installation *workspaceInstallation) RestoreDatabase(ctx context.Context, dump io.Reader) error {
	return installation.compose(ctx, composeCall{
		args:  append([]string{"exec", "-T", databaseService, "pg_restore", "--clean", "--if-exists", "--single-transaction"}, installation.databaseArgs()...),
		stdin: dump,
	})
}

// RefreshConfiguration is what `genwork up` does before starting: new .env keys and secrets, then the gateway routes.
func (installation *workspaceInstallation) RefreshConfiguration(context.Context) error {
	workspace := installation.workspace
	if err := workspace.EnsureEnvFiles(installation.stderr); err != nil {
		return err
	}
	version, err := workspace.Version()
	if err != nil {
		return err
	}
	catalog, _, err := modules.LoadModuleCatalog(workspace.Root, version)
	if err != nil {
		return err
	}
	config, err := modules.ReadInstallationConfig(workspace.InstallationFile())
	if err != nil {
		return err
	}
	if err := workspace.EnsureModuleSecrets(installation.stderr, config.Modules); err != nil {
		return err
	}
	return writeGatewayConfig(workspace, catalog, config)
}

// StartStack leaves the updater out: the updater may be the process running this update, and it replaces itself last.
func (installation *workspaceInstallation) StartStack(ctx context.Context, version string) error {
	services, err := installation.servicesExcept(ctx, version, updaterService)
	if err != nil {
		return err
	}
	args := append([]string{"up", "-d", "--wait", "--wait-timeout", stackStartupTimeout, "--remove-orphans"}, services...)
	return installation.composeWithProfiles(ctx, composeCall{version: version, args: args})
}

func (installation *workspaceInstallation) StopServicesExceptDatabase(ctx context.Context, version string) error {
	services, err := installation.servicesExcept(ctx, version, updaterService, databaseService)
	if err != nil {
		return err
	}
	return installation.composeWithProfiles(ctx, composeCall{version: version, args: append([]string{"stop"}, services...)})
}

func (installation *workspaceInstallation) ReportedVersion(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, installation.gatewayURL+"/.well-known/genwork", nil)
	if err != nil {
		return "", err
	}
	response, err := installation.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the gateway answered %s", response.Status)
	}
	var discovery struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&discovery); err != nil {
		return "", fmt.Errorf("decode /.well-known/genwork: %w", err)
	}
	return discovery.Version, nil
}

// replaceUpdater recreates the updater with version's image. Inside the updater it hands over to a one-off container
// of the new image, which survives the recreate because it is not the service's container (design §8).
func (installation *workspaceInstallation) replaceUpdater(ctx context.Context, version string) error {
	args := []string{"up", "-d", "--no-deps", "--force-recreate", updaterService}
	if installation.insideUpdater {
		args = []string{"run", "-d", "--rm", "--no-deps", updaterService, "genwork", "updater", "replace"}
	}
	return installation.compose(ctx, composeCall{version: version, args: args})
}

func (installation *workspaceInstallation) servicesExcept(ctx context.Context, version string, excluded ...string) ([]string, error) {
	var listed bytes.Buffer
	call := composeCall{version: version, args: []string{"config", "--services"}, stdout: &listed}
	if err := installation.composeWithProfiles(ctx, call); err != nil {
		return nil, err
	}
	var services []string
	for _, service := range strings.Fields(listed.String()) {
		if !slices.Contains(excluded, service) {
			services = append(services, service)
		}
	}
	return services, nil
}

// composeWithProfiles addresses every installed module's services, as the stack commands do.
func (installation *workspaceInstallation) composeWithProfiles(ctx context.Context, call composeCall) error {
	config, err := modules.ReadInstallationConfig(installation.workspace.InstallationFile())
	if err != nil {
		return err
	}
	var profiled []string
	for _, id := range config.Modules {
		profiled = append(profiled, "--profile", modules.ConventionsFor(id).Profile)
	}
	call.args = append(profiled, call.args...)
	return installation.compose(ctx, call)
}

func (installation *workspaceInstallation) databaseArgs() []string {
	return []string{"-U", installation.workspace.StackEnvValue("POSTGRES_USER"), "-d", installation.workspace.StackEnvValue("POSTGRES_DB")}
}

func (installation *workspaceInstallation) compose(ctx context.Context, call composeCall) error {
	if call.stdout == nil {
		call.stdout = installation.stdout
	}
	return runDockerCompose(ctx, installation.dependencies, installation.workspace, call, installation.stderr)
}
