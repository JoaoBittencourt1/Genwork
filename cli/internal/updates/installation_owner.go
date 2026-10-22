package updates

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// installationOwner is the owner of the installation directory. The updater container runs as root, so what it
// creates there would otherwise lock the operator's own `genwork update` out of the status, the lock, and the backups.
type installationOwner struct {
	uid, gid int
	active   bool
}

func findInstallationOwner(root string, effectiveUID int) installationOwner {
	if effectiveUID != 0 {
		return installationOwner{}
	}
	info, err := os.Stat(root)
	if err != nil {
		return installationOwner{}
	}
	uid, gid := ownerOf(info)
	return installationOwner{uid: uid, gid: gid, active: uid != 0}
}

func ownerOf(info fs.FileInfo) (int, int) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(stat.Uid), int(stat.Gid)
	}
	return 0, 0
}

// claim gives path to the installation's owner; it does nothing unless the process runs as root.
func (owner installationOwner) claim(paths ...string) error {
	if !owner.active {
		return nil
	}
	for _, path := range paths {
		if err := os.Lchown(path, owner.uid, owner.gid); err != nil {
			return err
		}
	}
	return nil
}

// claimTree claims a directory and everything in it, e.g. the snapshot a failed rollback leaves for the operator.
func (owner installationOwner) claimTree(root string) error {
	if !owner.active {
		return nil
	}
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return owner.claim(path)
	})
}
