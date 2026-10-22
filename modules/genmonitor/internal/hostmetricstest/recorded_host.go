// Package hostmetricstest holds a recorded Linux host and scripted readings for genmonitor's tests (rule 26).
package hostmetricstest

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// What the files under testdata/ describe; tests compare readings against these instead of restating the files.
const (
	RecordedLogicalCPUs          = 2
	RecordedCPUBusyTicks         = 1500
	RecordedCPUTotalTicks        = 10000
	RecordedMemoryTotalBytes     = 2048000 * 1024
	RecordedMemoryAvailableBytes = 1536000 * 1024
	RecordedInterface            = "eth0"
	RecordedReceivedBytes        = 1833748938
	RecordedTransmittedBytes     = 59364836
	RecordedUptime               = 93784 * time.Second
	RecordedOperatingSystemName  = "Ubuntu 24.04.4 LTS"
	RecordedKernelRelease        = "6.8.0-85-generic"
	RecordedHostname             = "vps-genwork"
)

var testdataPath = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}()

func ProcPath() string { return filepath.Join(testdataPath, "proc") }

func RootPath() string { return filepath.Join(testdataPath, "root") }

// NewProcTree copies the recorded procfs and replaces the given files, so a test describes only what is odd about its host.
func NewProcTree(t testing.TB, replacements map[string]string) string {
	return copyTreeWithReplacements(t, ProcPath(), replacements)
}

func NewRootTree(t testing.TB, replacements map[string]string) string {
	return copyTreeWithReplacements(t, RootPath(), replacements)
}

func copyTreeWithReplacements(t testing.TB, source string, replacements map[string]string) string {
	t.Helper()
	target := t.TempDir()
	if err := os.CopyFS(target, os.DirFS(source)); err != nil {
		t.Fatalf("copy %s: %v", source, err)
	}
	for relativePath, content := range replacements {
		path := filepath.Join(target, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), fs.FileMode(0o644)); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return target
}
