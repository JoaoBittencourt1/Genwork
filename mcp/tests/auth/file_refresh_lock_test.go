package auth_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
)

func TestFileRefreshLockIsExclusiveUntilReleased(t *testing.T) {
	lock := auth.NewFileRefreshLock(filepath.Join(t.TempDir(), "nested", "refresh.lock"))
	release, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	waiting, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := lock.Acquire(waiting); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second Acquire err = %v, want a timeout while the lock is held", err)
	}

	release()
	releaseAgain, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	releaseAgain()
}

func TestFileRefreshLockTakesOverALockLeftByADeadProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refresh.lock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("age lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := auth.NewFileRefreshLock(path).Acquire(ctx)

	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
}

func TestFileRefreshLockReportsADirectoryItCannotCreate(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	if _, err := auth.NewFileRefreshLock(filepath.Join(blocker, "refresh.lock")).Acquire(context.Background()); err == nil {
		t.Fatal("Acquire succeeded under a regular file")
	}
}
