package updates

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

func latestRelease(t *testing.T, fixture *runnerFixture) (releases.ReleaseChannel, releases.Release) {
	t.Helper()
	channel := fixture.runner.channel
	release, err := channel.FindLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return channel, release
}

func TestReplaceExecutableSwapsTheBinaryForTheVerifiedOne(t *testing.T) {
	fixture := newRunnerFixture(t)
	channel, release := latestRelease(t, fixture)
	executable := filepath.Join(t.TempDir(), "genwork")
	writeFile(t, executable, "old binary")

	err := ReplaceExecutable(context.Background(), channel, release, executable, runtime.GOOS, runtime.GOARCH)

	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	content, _ := os.ReadFile(executable)
	info, _ := os.Stat(executable)
	if string(content) != releasestest.ExecutableContent || info.Mode().Perm() != 0o755 {
		t.Errorf("binary = %q, mode = %v", content, info.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(executable)); len(entries) != 1 {
		t.Errorf("only the binary may remain next to it: %v", entries)
	}
}

func TestReplaceExecutableRefusesATamperedBinary(t *testing.T) {
	fixture := newRunnerFixture(t)
	fixture.github.TamperAsset(releasestest.ExecutableAsset(runtime.GOOS, runtime.GOARCH))
	channel, release := latestRelease(t, fixture)
	executable := filepath.Join(t.TempDir(), "genwork")
	writeFile(t, executable, "old binary")

	err := ReplaceExecutable(context.Background(), channel, release, executable, runtime.GOOS, runtime.GOARCH)

	if releases.ErrorCode(err) != releases.CodeChecksumMismatch {
		t.Errorf("error = %v", err)
	}
	if content, _ := os.ReadFile(executable); string(content) != "old binary" {
		t.Errorf("binary = %q", content)
	}
}

func TestReplaceExecutableLeavesAProtectedBinaryUnchanged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to read-only directories")
	}
	fixture := newRunnerFixture(t)
	channel, release := latestRelease(t, fixture)
	directory := t.TempDir()
	executable := filepath.Join(directory, "genwork")
	writeFile(t, executable, "old binary")
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })

	err := ReplaceExecutable(context.Background(), channel, release, executable, runtime.GOOS, runtime.GOARCH)

	if !errors.Is(err, ErrExecutableNotWritable) {
		t.Errorf("error = %v", err)
	}
	if content, _ := os.ReadFile(executable); string(content) != "old binary" {
		t.Errorf("binary = %q", content)
	}
}

func TestReplaceExecutableWithoutABinaryForThisPlatform(t *testing.T) {
	fixture := newRunnerFixture(t)
	channel, release := latestRelease(t, fixture)

	err := ReplaceExecutable(context.Background(), channel, release, filepath.Join(t.TempDir(), "genwork"), "plan9", "amd64")

	if err == nil {
		t.Error("expected an error")
	}
}
