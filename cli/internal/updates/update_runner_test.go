package updates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

func TestUpdateRunsEveryStepInOrderAndRecordsTheNewVersionLast(t *testing.T) {
	fixture := newRunnerFixture(t)

	status := fixture.update(t)

	if status.State != StateSucceeded || status.InstalledVersion != releasestest.LatestVersion || status.Failure != nil || status.FinishedAt == nil {
		t.Fatalf("status = %+v", status)
	}
	expectedCalls := []string{"pull 0.5.2", "dump", "refresh", "start 0.5.2"}
	if calls := fixture.installation.recordedCalls(); !slices.Equal(calls, expectedCalls) {
		t.Errorf("calls = %q, want %q", calls, expectedCalls)
	}
	var reported []string
	for _, line := range strings.Split(strings.TrimSpace(fixture.progress.String()), "\n") {
		reported = append(reported, strings.Fields(line)[0])
	}
	if !slices.Equal(reported, []string{"download", "pull", "backup", "swap", "migrate", "verify", "succeeded"}) {
		t.Errorf("progress = %q", fixture.progress.String())
	}
	if version := fixture.installation.readFile(t, "VERSION"); version != releasestest.LatestVersion+"\n" {
		t.Errorf("VERSION = %q", version)
	}
}

func TestUpdateSwapsTheDeployFilesAndKeepsTheConfiguration(t *testing.T) {
	fixture := newRunnerFixture(t)

	fixture.update(t)

	bundle := releasestest.BundleFiles()
	for _, path := range []string{"deploy/docker-compose.yml", "deploy/.env.example", "modules/gentasks/module.yaml", "modules/reporting/module.yaml"} {
		if got := fixture.installation.readFile(t, path); got != bundle[path] {
			t.Errorf("%s = %q, want the bundle's", path, got)
		}
	}
	for _, path := range []string{"deploy/.env", "deploy/genwork.yaml"} {
		if got := fixture.installation.readFile(t, path); got != installationFiles[path] {
			t.Errorf("%s must be kept, got %q", path, got)
		}
	}
	if _, err := os.Stat(filepath.Join(fixture.installation.root, ".genwork", "staging")); err == nil {
		t.Error("the staging directory must be removed after a successful update")
	}
}

func TestUpdateBacksUpTheDatabaseUnderThePreviousVersion(t *testing.T) {
	fixture := newRunnerFixture(t)

	fixture.update(t)

	backups := backupNames(t, fixture.installation.root)
	if !slices.Equal(backups, []string{"20261022T135500Z-0.5.0.dump"}) {
		t.Fatalf("backups = %v", backups)
	}
	content, _ := os.ReadFile(filepath.Join(fixture.installation.root, "backups", backups[0]))
	if string(content) != "dump of 0.5.0" {
		t.Errorf("backup = %q", content)
	}
}

func TestBeginRefusesWhenTheInstallationRunsTheLatestRelease(t *testing.T) {
	fixture := newRunnerFixture(t)
	writeFile(t, filepath.Join(fixture.installation.root, "VERSION"), releasestest.LatestVersion+"\n")
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := fixture.runner.Begin(releasestest.LatestVersion)

	if ErrorCode(err) != CodeNoNewerRelease || len(fixture.installation.recordedCalls()) != 0 {
		t.Errorf("error = %v, calls = %q", err, fixture.installation.recordedCalls())
	}
}

func TestBeginRefusesAVersionOtherThanTheLatestKnownRelease(t *testing.T) {
	fixture := newRunnerFixture(t)
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := fixture.runner.Begin("0.5.1")

	if ErrorCode(err) != CodeVersionMismatch {
		t.Errorf("error = %v", err)
	}
	if status, _ := fixture.runner.Status(); status.State != StateIdle {
		t.Errorf("a refused update must not change the status: %+v", status)
	}
}

func TestBeginRefusesBeforeAnyCheck(t *testing.T) {
	fixture := newRunnerFixture(t)

	if _, err := fixture.runner.Begin(releasestest.LatestVersion); ErrorCode(err) != CodeVersionMismatch {
		t.Errorf("error = %v", err)
	}
}

func TestChecksumMismatchStopsBeforeTheBackup(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.github.TamperAsset(releasestest.BundleAsset())

	status := fixture.update(t)

	if status.State != StateFailed || status.Failure.Code != "checksum_mismatch" || status.Failure.Step != StepDownload || status.Failure.RolledBack {
		t.Errorf("status = %+v, failure = %+v", status, status.Failure)
	}
	if calls := fixture.installation.recordedCalls(); len(calls) != 0 {
		t.Errorf("nothing may run after a checksum mismatch: %q", calls)
	}
	fixture.expectUntouched(t)
}

func TestPullFailureLeavesEverythingUntouched(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.installation.failures["pull 0.5.2"] = true

	status := fixture.update(t)

	if status.State != StateFailed || status.Failure.Step != StepPull || status.Failure.RolledBack || status.InstalledVersion != installedVersion {
		t.Errorf("status = %+v, failure = %+v", status, status.Failure)
	}
	if calls := fixture.installation.recordedCalls(); !slices.Equal(calls, []string{"pull 0.5.2"}) {
		t.Errorf("calls = %q", calls)
	}
	if backups := backupNames(t, fixture.installation.root); len(backups) != 0 {
		t.Errorf("no backup is taken before the images are pulled: %v", backups)
	}
	fixture.expectUntouched(t)
}

func TestMigrationFailureRestoresTheFilesAndTheDatabase(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.installation.failures["start 0.5.2"] = true

	status := fixture.update(t)

	if status.State != StateFailed || status.Failure.Step != StepMigrate || !status.Failure.RolledBack || status.InstalledVersion != installedVersion {
		t.Fatalf("status = %+v, failure = %+v", status, status.Failure)
	}
	expectedCalls := []string{"pull 0.5.2", "dump", "refresh", "start 0.5.2", "stop 0.5.0", "restore", "start 0.5.0"}
	if calls := fixture.installation.recordedCalls(); !slices.Equal(calls, expectedCalls) {
		t.Errorf("calls = %q, want %q", calls, expectedCalls)
	}
	if fixture.installation.restoredDump != "dump of 0.5.0" {
		t.Errorf("restored = %q, want this update's backup", fixture.installation.restoredDump)
	}
	fixture.expectUntouched(t)
}

func TestFailedRollbackReportsTheBackupToRestoreByHand(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.installation.failures["start 0.5.2"] = true
	fixture.installation.failures["restore"] = true

	status := fixture.update(t)

	if status.State != StateFailed || status.Failure.Step != StepMigrate || status.Failure.RolledBack {
		t.Fatalf("status = %+v, failure = %+v", status, status.Failure)
	}
	if _, err := os.Stat(status.Failure.BackupPath); err != nil || !strings.HasSuffix(status.Failure.BackupPath, "-0.5.0.dump") {
		t.Errorf("backup path = %q (%v)", status.Failure.BackupPath, err)
	}
}

func TestVerifyFailsWhenTheGatewayNeverReportsTheNewVersion(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.installation.stuckVersion = installedVersion

	status := fixture.update(t)

	if status.State != StateFailed || status.Failure.Step != StepVerify || !status.Failure.RolledBack {
		t.Errorf("status = %+v, failure = %+v", status, status.Failure)
	}
	fixture.expectUntouched(t)
}

func TestBackupsArePrunedToTheNewestFiveOnlyAfterSuccess(t *testing.T) {
	fixture := newRunnerFixture(t)
	for day := 1; day <= 5; day++ {
		writeFile(t, filepath.Join(fixture.installation.root, "backups", fmt.Sprintf("202610%02dT000000Z-0.4.0.dump", day)), "old")
	}
	fixture.installation.failures["start 0.5.2"] = true
	fixture.update(t)
	if backups := backupNames(t, fixture.installation.root); len(backups) != 6 {
		t.Fatalf("a failed update must not prune: %v", backups)
	}

	delete(fixture.installation.failures, "start 0.5.2")
	fixture.update(t)

	backups := backupNames(t, fixture.installation.root)
	if len(backups) != 5 || slices.Contains(backups, "20261001T000000Z-0.4.0.dump") || !slices.Contains(backups, "20261022T135500Z-0.5.0.dump") {
		t.Errorf("backups = %v", backups)
	}
}

func TestOnlyOneUpdateOrCheckRunsAtATime(t *testing.T) {
	fixture := newRunnerFixture(t)
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, err := fixture.runner.Begin(releasestest.LatestVersion)
	if err != nil {
		t.Fatal(err)
	}
	other := fixture.newRunner()

	_, beginErr := other.Begin(releasestest.LatestVersion)
	_, checkErr := other.Check(context.Background())

	if ErrorCode(beginErr) != CodeUpdateInProgress || ErrorCode(checkErr) != CodeUpdateInProgress {
		t.Errorf("begin = %v, check = %v", beginErr, checkErr)
	}
	if status, _ := other.Status(); status.State != StateUpdating || status.TargetVersion != releasestest.LatestVersion {
		t.Errorf("status while running = %+v", status)
	}
	running.Run(context.Background())
	if _, err := other.Check(context.Background()); err != nil {
		t.Errorf("the lock must be released after the update: %v", err)
	}
}

func TestRepositoryCheckoutsAreNotUpdatable(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.installation.repository = true

	status, err := fixture.runner.Check(context.Background())
	if err != nil || status.Latest == nil || status.Updatable || status.Reason != ReasonDevelopmentInstallation {
		t.Fatalf("check = %+v, %v", status, err)
	}
	_, err = fixture.runner.Begin(releasestest.LatestVersion)

	if ErrorCode(err) != ReasonDevelopmentInstallation || len(fixture.installation.recordedCalls()) != 0 {
		t.Errorf("error = %v", err)
	}
}

func TestCheckRecordsTheLatestRelease(t *testing.T) {
	fixture := newRunnerFixture(t)

	status, err := fixture.runner.Check(context.Background())

	if err != nil || status.Latest == nil || status.Latest.Version != releasestest.LatestVersion ||
		status.Latest.NotesURL != "https://github.com/acme/genwork/releases/tag/v0.5.2" || status.CheckedAt == nil || !status.CheckedAt.Equal(fixedNow) {
		t.Fatalf("status = %+v, %v", status, err)
	}
	if !status.Updatable || status.State != StateIdle || status.InstalledVersion != installedVersion {
		t.Errorf("status = %+v", status)
	}
}

func TestFailedCheckKeepsTheRecordedRelease(t *testing.T) {
	fixture := newRunnerFixture(t)
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	fixture.github.RateLimit()

	_, err := fixture.runner.Check(context.Background())

	if ErrorCode(err) != "release_channel_rate_limited" {
		t.Errorf("error = %v", err)
	}
	if status, _ := fixture.runner.Status(); status.Latest == nil || status.Latest.Version != releasestest.LatestVersion {
		t.Errorf("status = %+v", status)
	}
}

func TestCheckWithoutAnyPublishedRelease(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.github.RespondToLatest(404, "not_found.json", nil)

	status, err := fixture.runner.Check(context.Background())

	if err != nil || status.Latest != nil || status.CheckedAt == nil {
		t.Errorf("status = %+v, %v", status, err)
	}
}

func TestStatusSurvivesARestart(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.update(t)

	restarted, err := fixture.newRunner().Status()

	if err != nil || restarted.State != StateSucceeded || restarted.InstalledVersion != releasestest.LatestVersion || restarted.TargetVersion != releasestest.LatestVersion {
		t.Errorf("status = %+v, %v", restarted, err)
	}
}

func TestAnUpdateWhoseProcessDiedIsReportedAsInterrupted(t *testing.T) {
	fixture := newRunnerFixture(t)
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, err := fixture.runner.Begin(releasestest.LatestVersion)
	if err != nil {
		t.Fatal(err)
	}
	running.lock.Release()

	status, err := fixture.newRunner().Status()

	if err != nil || status.State != StateFailed || status.Failure == nil || status.Failure.Code != CodeUpdateInterrupted {
		t.Errorf("status = %+v, %v", status, err)
	}
}

func TestStatusOfAFreshInstallation(t *testing.T) {
	fixture := newRunnerFixture(t)

	status, err := fixture.runner.Status()

	if err != nil || status.State != StateIdle || status.Latest != nil || status.CheckedAt != nil || status.InstalledVersion != installedVersion {
		t.Errorf("status = %+v, %v", status, err)
	}
}

func TestStatusFailsWithoutAVersionFile(t *testing.T) {
	fixture := newRunnerFixture(t)
	if err := os.Remove(filepath.Join(fixture.installation.root, "VERSION")); err != nil {
		t.Fatal(err)
	}

	if _, err := fixture.runner.Status(); err == nil || errors.Is(err, nil) {
		t.Error("expected an error")
	}
}
