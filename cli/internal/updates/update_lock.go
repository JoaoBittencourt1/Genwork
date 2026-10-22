package updates

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// UpdateLock is one flock per installation, shared by the host CLI and the updater through the bind-mounted directory.
type UpdateLock struct {
	file *os.File
}

// acquireLock never waits: a second update or check is refused with update_in_progress instead. flock needs no write
// access, so the lock file of the root-run updater locks the operator's CLI as well.
func acquireLock(path string) (*UpdateLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, newUpdateError(CodeUpdateInProgress, nil)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &UpdateLock{file: file}, nil
}

// Release is safe to call more than once.
func (lock *UpdateLock) Release() {
	if lock.file != nil {
		_ = lock.file.Close()
		lock.file = nil
	}
}
