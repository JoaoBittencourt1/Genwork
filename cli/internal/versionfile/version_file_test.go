package versionfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTrimsTheVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("0.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	version, err := Read(path)

	if err != nil || version != "0.5.0" {
		t.Errorf("version = %q, err = %v", version, err)
	}
}

func TestReadRejectsAMissingFileOrAnInvalidVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "VERSION") {
		t.Errorf("missing file: error = %v", err)
	}
	for _, content := range []string{"", "v0.5.0", "0.5", "latest"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "VERSION") {
			t.Errorf("%q: error = %v", content, err)
		}
	}
}

func TestWriteReplacesTheVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("0.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Write(path, "0.5.2"); err != nil {
		t.Fatalf("write: %v", err)
	}

	if content, _ := os.ReadFile(path); string(content) != "0.5.2\n" {
		t.Errorf("content = %q", content)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("no temporary file may remain: %v", entries)
	}
}

func TestWriteRejectsAnInvalidVersionOrDirectory(t *testing.T) {
	if err := Write(filepath.Join(t.TempDir(), "VERSION"), "next"); err == nil {
		t.Error("an invalid version must be refused")
	}
	if err := Write(filepath.Join(t.TempDir(), "missing", "VERSION"), "0.5.2"); err == nil {
		t.Error("a missing directory must fail")
	}
}
