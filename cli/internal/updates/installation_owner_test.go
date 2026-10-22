package updates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilesKeepTheirOwnerUnlessTheUpdaterRunsAsRoot(t *testing.T) {
	root := t.TempDir()
	owner := findInstallationOwner(root, 1000)
	if owner.active {
		t.Error("a process that is not root creates files as itself, already owned right")
	}
	path := filepath.Join(root, "created")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := owner.claim(path); err != nil {
		t.Errorf("claim must be a no-op without root: %v", err)
	}
}

func TestRootHandsFilesToTheInstallationsOwner(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	owner := findInstallationOwner(root, 0)
	uid, gid := ownerOf(info)
	if owner.active != (uid != 0) || owner.uid != uid || owner.gid != gid {
		t.Errorf("owner = %+v, directory owner = %d:%d", owner, uid, gid)
	}
}

func TestTheLockOpensReadOnlySoAnyUserCanTakeIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".update.lock")
	if err := os.WriteFile(path, nil, 0o444); err != nil {
		t.Fatal(err)
	}

	lock, err := acquireLock(path)

	if err != nil {
		t.Fatalf("a lock file this user cannot write must still lock: %v", err)
	}
	lock.Release()
}
