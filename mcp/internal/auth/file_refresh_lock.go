package auth

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultLockStaleAfter = 30 * time.Second
	DefaultLockRetryEvery = 50 * time.Millisecond
)

// FileRefreshLock is an exclusive-create lock file, portable across macOS, Windows, and Linux.
// A lock older than StaleAfter belongs to a process that died mid-refresh and is taken over.
type FileRefreshLock struct {
	Path       string
	StaleAfter time.Duration
	RetryEvery time.Duration
}

func NewFileRefreshLock(path string) *FileRefreshLock {
	return &FileRefreshLock{Path: path, StaleAfter: DefaultLockStaleAfter, RetryEvery: DefaultLockRetryEvery}
}

func (lock *FileRefreshLock) Acquire(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lock.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	for {
		acquired, err := lock.tryCreate()
		if err != nil {
			return nil, err
		}
		if acquired {
			return func() { _ = os.Remove(lock.Path) }, nil
		}
		lock.removeIfStale()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for the refresh lock: %w", ctx.Err())
		case <-time.After(lock.RetryEvery):
		}
	}
}

func (lock *FileRefreshLock) tryCreate() (bool, error) {
	file, err := os.OpenFile(lock.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create refresh lock: %w", err)
	}
	return true, file.Close()
}

func (lock *FileRefreshLock) removeIfStale() {
	info, err := os.Stat(lock.Path)
	if err == nil && time.Since(info.ModTime()) > lock.StaleAfter {
		_ = os.Remove(lock.Path)
	}
}
