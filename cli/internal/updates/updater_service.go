package updates

import (
	"context"
	"time"
)

const checkTimeout = 15 * time.Second

// UpdaterService is what the updater container serves to core: it owns the latest known release, so core never
// duplicates the version check (design §8).
type UpdaterService struct {
	runner *UpdateRunner
	// afterSuccess replaces the updater's own container, last, once the new version is recorded.
	afterSuccess func(ctx context.Context, status UpdateStatus)
}

func NewUpdaterService(runner *UpdateRunner, afterSuccess func(ctx context.Context, status UpdateStatus)) *UpdaterService {
	return &UpdaterService{runner: runner, afterSuccess: afterSuccess}
}

func (service *UpdaterService) Status() (UpdateStatus, error) {
	return service.runner.Status()
}

func (service *UpdaterService) Check(ctx context.Context) (UpdateStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return service.runner.Check(ctx)
}

// StartUpdate answers as soon as the update holds the lock; the update itself outlives the request.
func (service *UpdaterService) StartUpdate(version string) (UpdateStatus, error) {
	running, err := service.runner.Begin(version)
	if err != nil {
		return UpdateStatus{}, err
	}
	status, err := running.Status()
	if err != nil {
		running.lock.Release()
		return UpdateStatus{}, err
	}
	go func() {
		ctx := context.Background()
		if final := running.Run(ctx); final.State == StateSucceeded {
			service.afterSuccess(ctx, final)
		}
	}()
	return status, nil
}
