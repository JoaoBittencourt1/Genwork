package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/updates"
	"github.com/spf13/cobra"
)

var ErrUpdateInRepository = errors.New("a repository checkout updates through git: pull the new commits, then run `genwork up`")

func newUpdateCommand(dependencies Dependencies) *cobra.Command {
	var checkOnly bool
	command := &cobra.Command{
		Use:   "update",
		Short: "Install the latest Genwork release, with a database backup first and a rollback if it fails",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runUpdate(command, dependencies, checkOnly)
		},
	}
	command.Flags().BoolVar(&checkOnly, "check", false, "only report whether a newer release exists")
	return command
}

// runUpdate checks first, so `--check` and a refused update both leave the installation untouched.
func runUpdate(command *cobra.Command, dependencies Dependencies, checkOnly bool) error {
	workspace, err := locateWorkspace(dependencies)
	if err != nil {
		return err
	}
	if !checkOnly && workspace.Kind() == WorkspaceRepository {
		return ErrUpdateInRepository
	}
	channel, err := newReleaseChannel(workspace)
	if err != nil {
		return err
	}
	output := command.ErrOrStderr()
	installation := newWorkspaceInstallation(dependencies, workspace, command.OutOrStdout(), output,
		"http://localhost:"+workspace.StackEnvValue("GATEWAY_PORT"))
	runner := updates.NewUpdateRunner(updates.UpdateRunnerConfig{Installation: installation, Channel: channel, Now: time.Now, Progress: output})
	status, err := runner.Check(command.Context())
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	newer := printCheckResult(output, status)
	if checkOnly {
		return nil
	}
	if !newer {
		return replaceOutdatedExecutable(command, dependencies, channel, status.InstalledVersion)
	}
	running, err := runner.Begin(status.Latest.Version)
	if err != nil {
		return err
	}
	final := running.Run(command.Context())
	if final.State != updates.StateSucceeded {
		printUpdateFailure(output, workspace, final)
		return ErrReported
	}
	if err := installation.replaceUpdater(command.Context(), final.InstalledVersion); err != nil {
		fmt.Fprintln(output, "Warning: the updater still runs the previous version; run `genwork up` to replace it.")
	}
	return replaceOutdatedExecutable(command, dependencies, channel, final.InstalledVersion)
}

func newReleaseChannel(workspace Workspace) (*releases.GithubReleaseChannel, error) {
	apiURL, repository := workspace.StackEnvValue("GITHUB_API_URL"), workspace.StackEnvValue("GENWORK_RELEASE_REPOSITORY")
	if missing := missingVariables(workspace.StackEnvValue, "GITHUB_API_URL", "GENWORK_RELEASE_REPOSITORY"); len(missing) > 0 {
		return nil, fmt.Errorf("set %s in %s to check for releases", strings.Join(missing, " and "), workspace.StackEnvFile())
	}
	return releases.NewGithubReleaseChannel(releases.GithubReleaseChannelConfig{
		APIURL: apiURL, Repository: repository, UserAgent: "genwork/" + Version,
	}), nil
}

// printCheckResult reports the check and tells whether a newer release can be installed.
func printCheckResult(output io.Writer, status updates.UpdateStatus) bool {
	latest := status.Latest
	if latest == nil || !(releases.Release{Version: latest.Version}).IsNewerThan(status.InstalledVersion) {
		fmt.Fprintf(output, "Genwork %s is up to date.\n", status.InstalledVersion)
		return false
	}
	fmt.Fprintf(output, "Genwork %s is available (installed: %s), published %s.\nRelease notes: %s\n",
		latest.Version, status.InstalledVersion, latest.PublishedAt.Format("2006-01-02"), latest.NotesURL)
	return true
}

func printUpdateFailure(output io.Writer, workspace Workspace, status updates.UpdateStatus) {
	failure := status.Failure
	fmt.Fprintf(output, "The update to %s failed at step %s (%s).\n", status.TargetVersion, failure.Step, failure.Code)
	switch {
	case failure.RolledBack:
		fmt.Fprintf(output, "Genwork is back on %s, with the database restored from the backup taken before the update.\n", status.PreviousVersion)
	case failure.BackupPath == "":
		fmt.Fprintf(output, "Nothing was changed; Genwork still runs %s.\n", status.InstalledVersion)
	default:
		compose := "GENWORK_VERSION=" + status.PreviousVersion + " docker compose -f " + workspace.ComposeFile()
		fmt.Fprintf(output, "Restoring %s failed too. The database backup is %s. To recover by hand:\n"+
			"  %s stop\n  %s up -d --wait postgres\n"+
			"  %s exec -T postgres pg_restore --clean --if-exists --single-transaction -U %s -d %s < %s\n  %s up -d\n",
			status.PreviousVersion, failure.BackupPath, compose, compose, compose,
			workspace.StackEnvValue("POSTGRES_USER"), workspace.StackEnvValue("POSTGRES_DB"), failure.BackupPath, compose)
	}
}

// replaceOutdatedExecutable brings this binary to the installed version, after an update from here or from the web app.
func replaceOutdatedExecutable(command *cobra.Command, dependencies Dependencies, channel releases.ReleaseChannel, installedVersion string) error {
	if !(releases.Release{Version: installedVersion}).IsNewerThan(Version) || dependencies.Executable == nil {
		return nil
	}
	executable, err := dependencies.Executable()
	if err != nil {
		return err
	}
	release, err := channel.FindLatest(command.Context())
	if err != nil {
		return fmt.Errorf("find the genwork binary of %s: %w", installedVersion, err)
	}
	if release.Version != installedVersion {
		return nil
	}
	err = updates.ReplaceExecutable(command.Context(), channel, release, executable, runtime.GOOS, runtime.GOARCH)
	output := command.ErrOrStderr()
	if errors.Is(err, updates.ErrExecutableNotWritable) {
		asset := releases.ExecutableAsset(release.Version, runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(output, "This genwork binary (%s) is still %s, since its directory is not writable. Replace it with:\n"+
			"  curl -fsSL %s | sudo tar -xzf - -C %s genwork\n", executable, Version, release.Assets[asset], filepath.Dir(executable))
		return nil
	}
	if err != nil {
		return fmt.Errorf("replace %s: %w", executable, err)
	}
	fmt.Fprintf(output, "Replaced %s with genwork %s.\n", executable, release.Version)
	return nil
}

// warnWhenBinaryIsOutdated runs before every command of a release installation, whose binary must match VERSION.
func warnWhenBinaryIsOutdated(command *cobra.Command, dependencies Dependencies) {
	workspace, err := locateWorkspace(dependencies)
	if err != nil || workspace.Kind() == WorkspaceRepository {
		return
	}
	installed, err := workspace.Version()
	if err != nil || !(releases.Release{Version: installed}).IsNewerThan(Version) {
		return
	}
	fmt.Fprintf(command.ErrOrStderr(), "Warning: this genwork binary is %s, but the installation runs %s. Run `genwork update` to replace it.\n",
		Version, installed)
}
