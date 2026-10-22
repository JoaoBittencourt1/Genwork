package updates

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

var ErrUpdaterUnavailable = platform.NewAPIError(http.StatusServiceUnavailable, "updater_unavailable")

// Codes the updater refuses with that admins can act on; anything else means the updater itself is unusable.
var (
	conflictCodes = []string{"version_mismatch", "update_in_progress", "no_newer_release", "development_installation"}
	channelCodes  = []string{"release_channel_unreachable", "release_channel_rate_limited"}
)

type UpdateService struct {
	updater UpdaterClient
}

func NewUpdateService(updater UpdaterClient) *UpdateService {
	return &UpdateService{updater: updater}
}

func (service *UpdateService) FindStatus(ctx context.Context) (UpdaterStatus, error) {
	status, err := service.updater.FindStatus(ctx)
	return status, translateUpdaterError(err)
}

func (service *UpdateService) CheckForUpdates(ctx context.Context) (UpdaterStatus, error) {
	status, err := service.updater.CheckForUpdates(ctx)
	return status, translateUpdaterError(err)
}

// StartUpdate leaves the version check to the updater, which owns the latest known release (design §8).
func (service *UpdateService) StartUpdate(ctx context.Context, version string) (UpdaterStatus, error) {
	status, err := service.updater.StartUpdate(ctx, version)
	return status, translateUpdaterError(err)
}

func translateUpdaterError(err error) error {
	if err == nil {
		return nil
	}
	var refusal *UpdaterRefusal
	switch {
	case errors.As(err, &refusal) && slices.Contains(conflictCodes, refusal.Code):
		return platform.NewAPIError(http.StatusConflict, refusal.Code)
	case errors.As(err, &refusal) && slices.Contains(channelCodes, refusal.Code):
		return platform.NewAPIError(http.StatusBadGateway, refusal.Code)
	default:
		return ErrUpdaterUnavailable
	}
}
