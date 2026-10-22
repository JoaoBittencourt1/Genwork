// Package updates installs the latest platform release on an installation, with a database backup first and an
// automatic rollback when a later step fails (design §7). The host CLI and the updater container share it.
package updates

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/versionfile"
)

const (
	keptBackups          = 5
	defaultVerifyTimeout = 2 * time.Minute
	defaultPollInterval  = 2 * time.Second
)

type UpdateRunnerConfig struct {
	Installation Installation
	Channel      releases.ReleaseChannel
	Now          func() time.Time
	// Progress receives one line per step, starting with the step's name.
	Progress io.Writer
	// VerifyTimeout bounds the wait for the gateway to report a version; zero means 2 minutes, polled every PollInterval.
	VerifyTimeout time.Duration
	PollInterval  time.Duration
}

type UpdateRunner struct {
	installation  Installation
	channel       releases.ReleaseChannel
	now           func() time.Time
	progress      io.Writer
	verifyTimeout time.Duration
	pollInterval  time.Duration
	store         statusStore
	owner         installationOwner
}

func NewUpdateRunner(config UpdateRunnerConfig) *UpdateRunner {
	owner := findInstallationOwner(config.Installation.Root(), os.Geteuid())
	runner := &UpdateRunner{
		installation: config.Installation, channel: config.Channel, now: config.Now, progress: config.Progress,
		verifyTimeout: config.VerifyTimeout, pollInterval: config.PollInterval, owner: owner,
		store: statusStore{path: filepath.Join(config.Installation.Root(), "deploy", "update-status.json"), owner: owner},
	}
	if runner.progress == nil {
		runner.progress = io.Discard
	}
	if runner.verifyTimeout == 0 {
		runner.verifyTimeout = defaultVerifyTimeout
	}
	if runner.pollInterval == 0 {
		runner.pollInterval = defaultPollInterval
	}
	return runner
}

func (runner *UpdateRunner) Status() (UpdateStatus, error) {
	recorded, err := runner.store.read()
	if err != nil {
		return UpdateStatus{}, err
	}
	if recorded.State == StateUpdating {
		if lock, err := runner.acquireLock(); err == nil {
			recorded, err = runner.settleInterrupted(recorded)
			lock.Release()
			if err != nil {
				return UpdateStatus{}, err
			}
		}
	}
	return runner.describe(recorded)
}

// Check records the latest release; a failed check changes nothing (spec "Checking for a newer release").
func (runner *UpdateRunner) Check(ctx context.Context) (UpdateStatus, error) {
	lock, err := runner.acquireLock()
	if err != nil {
		return UpdateStatus{}, err
	}
	defer lock.Release()
	recorded, err := runner.readSettled()
	if err != nil {
		return UpdateStatus{}, err
	}
	release, err := runner.channel.FindLatest(ctx)
	switch {
	case errors.Is(err, releases.ErrNoPublishedRelease):
		recorded.Latest = nil
	case err != nil:
		return UpdateStatus{}, err
	default:
		recorded.Latest = &LatestRelease{Version: release.Version, NotesURL: release.NotesURL, PublishedAt: release.PublishedAt}
	}
	checkedAt := runner.now()
	recorded.CheckedAt = &checkedAt
	if err := runner.store.write(recorded); err != nil {
		return UpdateStatus{}, err
	}
	return runner.describe(recorded)
}

// Begin takes the lock and refuses anything but the latest known release, newer than the installed one. The lock stays
// held until the returned update finishes, so the updater can answer before running it.
func (runner *UpdateRunner) Begin(targetVersion string) (*RunningUpdate, error) {
	lock, err := runner.acquireLock()
	if err != nil {
		return nil, err
	}
	recorded, err := runner.readSettled()
	if err == nil {
		err = runner.refuseUnlessApplicable(recorded, targetVersion)
	}
	if err != nil {
		lock.Release()
		return nil, err
	}
	installed, _ := versionfile.Read(runner.versionFile())
	startedAt := runner.now()
	recorded.State, recorded.TargetVersion, recorded.PreviousVersion = StateUpdating, targetVersion, installed
	recorded.Step, recorded.StartedAt, recorded.FinishedAt, recorded.Failure = StepDownload, &startedAt, nil, nil
	if err := runner.store.write(recorded); err != nil {
		lock.Release()
		return nil, err
	}
	return &RunningUpdate{runner: runner, lock: lock, recorded: recorded}, nil
}

func (runner *UpdateRunner) refuseUnlessApplicable(recorded RecordedStatus, targetVersion string) error {
	status, err := runner.describe(recorded)
	if err != nil {
		return err
	}
	if !status.Updatable {
		return newUpdateError(status.Reason, nil)
	}
	if recorded.Latest == nil || recorded.Latest.Version != targetVersion {
		return newUpdateError(CodeVersionMismatch, fmt.Errorf("v%s is not the latest known release", targetVersion))
	}
	if !(releases.Release{Version: targetVersion}).IsNewerThan(status.InstalledVersion) {
		return newUpdateError(CodeNoNewerRelease, fmt.Errorf("the installation already runs %s", status.InstalledVersion))
	}
	return nil
}

func (runner *UpdateRunner) describe(recorded RecordedStatus) (UpdateStatus, error) {
	installed, err := versionfile.Read(runner.versionFile())
	if err != nil {
		return UpdateStatus{}, err
	}
	status := UpdateStatus{InstalledVersion: installed, Updatable: true, RecordedStatus: recorded}
	if runner.installation.IsRepositoryCheckout() {
		status.Updatable, status.Reason = false, ReasonDevelopmentInstallation
	}
	return status, nil
}

func (runner *UpdateRunner) readSettled() (RecordedStatus, error) {
	recorded, err := runner.store.read()
	if err != nil {
		return RecordedStatus{}, err
	}
	return runner.settleInterrupted(recorded)
}

// settleInterrupted runs while holding the lock: an "updating" status nobody holds the lock for belongs to a process
// that died mid-update, and must not block every later update.
func (runner *UpdateRunner) settleInterrupted(recorded RecordedStatus) (RecordedStatus, error) {
	if recorded.State != StateUpdating {
		return recorded, nil
	}
	finishedAt := runner.now()
	recorded.State, recorded.FinishedAt = StateFailed, &finishedAt
	recorded.Failure = &UpdateFailure{Code: CodeUpdateInterrupted, Step: recorded.Step}
	return recorded, runner.store.write(recorded)
}

func (runner *UpdateRunner) acquireLock() (*UpdateLock, error) {
	path := filepath.Join(runner.installation.Root(), "deploy", ".update.lock")
	lock, err := acquireLock(path)
	if err != nil {
		return nil, err
	}
	if err := runner.owner.claim(path); err != nil {
		lock.Release()
		return nil, err
	}
	return lock, nil
}

func (runner *UpdateRunner) versionFile() string {
	return filepath.Join(runner.installation.Root(), "VERSION")
}

// waitForVersion polls the gateway until it reports version, since every container may still be starting.
func (runner *UpdateRunner) waitForVersion(ctx context.Context, version string) error {
	deadline := time.Now().Add(runner.verifyTimeout)
	for {
		reported, err := runner.installation.ReportedVersion(ctx)
		if err == nil && reported == version {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the gateway did not report %s within %s (last answer: %q, %v)", version, runner.verifyTimeout, reported, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(runner.pollInterval):
		}
	}
}

// RunningUpdate is an update that holds the installation's lock until Run returns.
type RunningUpdate struct {
	runner   *UpdateRunner
	lock     *UpdateLock
	recorded RecordedStatus
}

// Status is the status recorded when the update began.
func (update *RunningUpdate) Status() (UpdateStatus, error) {
	return update.runner.describe(update.recorded)
}

// Run never returns an error: every failure ends in the recorded status, which is what callers show.
func (update *RunningUpdate) Run(ctx context.Context) UpdateStatus {
	defer update.lock.Release()
	runner := update.runner
	root := runner.installation.Root()
	stagingRoot := filepath.Join(root, ".genwork", "staging")
	staging := filepath.Join(stagingRoot, update.recorded.TargetVersion)
	defer os.RemoveAll(stagingRoot)

	stagedRoot, err := update.download(ctx, staging)
	if err != nil {
		return update.fail(StepDownload, codeOr(err, CodeDownloadFailed), err)
	}
	update.enter(StepPull, "Pulling the images of v"+update.recorded.TargetVersion)
	if err := runner.installation.PullImages(ctx, update.recorded.TargetVersion, stagedRoot); err != nil {
		return update.fail(StepPull, CodePullFailed, err)
	}
	backupPath, err := update.backup(ctx)
	if err != nil {
		return update.fail(StepBackup, CodeBackupFailed, err)
	}
	snapshot := filepath.Join(root, ".genwork", "previous")
	update.enter(StepSwap, "Replacing the deploy files")
	if err := update.swap(ctx, stagedRoot, snapshot); err != nil {
		return update.rollBack(ctx, StepSwap, CodeSwapFailed, err, snapshot, backupPath)
	}
	update.enter(StepMigrate, "Running migrations and restarting the services")
	if err := runner.installation.StartStack(ctx, update.recorded.TargetVersion); err != nil {
		return update.rollBack(ctx, StepMigrate, CodeMigrationFailed, err, snapshot, backupPath)
	}
	update.enter(StepVerify, "Waiting for the gateway to report v"+update.recorded.TargetVersion)
	if err := runner.waitForVersion(ctx, update.recorded.TargetVersion); err != nil {
		return update.rollBack(ctx, StepVerify, CodeVerifyFailed, err, snapshot, backupPath)
	}
	if err := versionfile.Write(runner.versionFile(), update.recorded.TargetVersion); err != nil {
		return update.rollBack(ctx, StepVerify, CodeVerifyFailed, err, snapshot, backupPath)
	}
	if err := runner.owner.claim(runner.versionFile()); err != nil {
		return update.rollBack(ctx, StepVerify, CodeVerifyFailed, err, snapshot, backupPath)
	}
	return update.succeed(snapshot)
}

func (update *RunningUpdate) download(ctx context.Context, staging string) (string, error) {
	update.enter(StepDownload, "Downloading Genwork v"+update.recorded.TargetVersion)
	channel := update.runner.channel
	release, err := channel.FindLatest(ctx)
	if err != nil {
		return "", err
	}
	if release.Version != update.recorded.TargetVersion {
		return "", newUpdateError(CodeVersionMismatch, fmt.Errorf("the latest release is now v%s", release.Version))
	}
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", err
	}
	bundle := releases.BundleAsset(release.Version)
	for _, asset := range []string{releases.ChecksumsAsset, bundle} {
		if err := channel.Download(ctx, release, asset, filepath.Join(staging, asset)); err != nil {
			return "", err
		}
	}
	if err := releases.VerifyChecksums(staging, bundle); err != nil {
		return "", err
	}
	stagedRoot := filepath.Join(staging, "bundle")
	return stagedRoot, extractBundle(filepath.Join(staging, bundle), stagedRoot)
}

// backup names the dump after the version it restores to, with a UTC timestamp first so names sort by age.
func (update *RunningUpdate) backup(ctx context.Context) (path string, err error) {
	update.enter(StepBackup, "Backing up the database")
	directory := filepath.Join(update.runner.installation.Root(), "backups")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	name := update.runner.now().UTC().Format("20060102T150405Z") + "-" + update.recorded.PreviousVersion + ".dump"
	path = filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	if err := update.runner.owner.claim(directory, path); err != nil {
		file.Close()
		return "", err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return path, update.runner.installation.DumpDatabase(ctx, file)
}

func (update *RunningUpdate) swap(ctx context.Context, stagedRoot, snapshot string) error {
	root := update.runner.installation.Root()
	if err := snapshotDeployFiles(root, snapshot); err != nil {
		return err
	}
	if err := swapDeployFiles(stagedRoot, root); err != nil {
		return err
	}
	owner := update.runner.owner
	if err := errors.Join(owner.claimTree(filepath.Dir(snapshot)), owner.claimTree(filepath.Join(root, "modules"))); err != nil {
		return err
	}
	return update.runner.installation.RefreshConfiguration(ctx)
}

// rollBack runs even when ctx was cancelled: an interrupted CLI must still leave the previous version running.
func (update *RunningUpdate) rollBack(ctx context.Context, step UpdateStep, code string, cause error, snapshot, backupPath string) UpdateStatus {
	fmt.Fprintf(update.runner.progress, "rollback  %s failed (%v); restoring v%s\n", step, cause, update.recorded.PreviousVersion)
	ctx = context.WithoutCancel(ctx)
	installation := update.runner.installation
	previous := update.recorded.PreviousVersion
	err := restoreDeployFiles(snapshot, installation.Root())
	if err == nil {
		err = installation.StopServicesExceptDatabase(ctx, previous)
	}
	if err == nil {
		err = restoreDatabase(ctx, installation, backupPath)
	}
	if err == nil {
		err = installation.StartStack(ctx, previous)
	}
	if err == nil {
		err = update.runner.waitForVersion(ctx, previous)
	}
	failure := &UpdateFailure{Code: code, Step: step, RolledBack: err == nil}
	if err != nil {
		failure.BackupPath = backupPath
		fmt.Fprintf(update.runner.progress, "rollback  failed: %v\n", err)
	} else {
		_ = os.RemoveAll(snapshot)
	}
	return update.finish(StateFailed, failure)
}

func restoreDatabase(ctx context.Context, installation Installation, backupPath string) error {
	dump, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer dump.Close()
	return installation.RestoreDatabase(ctx, dump)
}

func (update *RunningUpdate) fail(step UpdateStep, code string, cause error) UpdateStatus {
	fmt.Fprintf(update.runner.progress, "failed  %s: %v; nothing was changed\n", step, cause)
	return update.finish(StateFailed, &UpdateFailure{Code: code, Step: step})
}

func (update *RunningUpdate) succeed(snapshot string) UpdateStatus {
	root := update.runner.installation.Root()
	if err := pruneBackups(filepath.Join(root, "backups"), keptBackups); err != nil {
		fmt.Fprintf(update.runner.progress, "warning  could not prune old backups: %v\n", err)
	}
	_ = os.RemoveAll(snapshot)
	fmt.Fprintf(update.runner.progress, "succeeded  Genwork v%s is running\n", update.recorded.TargetVersion)
	return update.finish(StateSucceeded, nil)
}

func (update *RunningUpdate) finish(state UpdateState, failure *UpdateFailure) UpdateStatus {
	finishedAt := update.runner.now()
	update.recorded.State, update.recorded.FinishedAt, update.recorded.Failure = state, &finishedAt, failure
	update.record()
	status, err := update.runner.describe(update.recorded)
	if err != nil {
		fmt.Fprintf(update.runner.progress, "warning  could not read VERSION: %v\n", err)
	}
	return status
}

func (update *RunningUpdate) enter(step UpdateStep, description string) {
	update.recorded.Step = step
	update.record()
	fmt.Fprintf(update.runner.progress, "%s  %s\n", step, description)
}

// record keeps going when the status cannot be written: stopping halfway would be worse than a stale status.
func (update *RunningUpdate) record() {
	if err := update.runner.store.write(update.recorded); err != nil {
		fmt.Fprintf(update.runner.progress, "warning  could not record the update status: %v\n", err)
	}
}

func codeOr(err error, fallback string) string {
	if code := ErrorCode(err); code != "" {
		return code
	}
	return fallback
}
