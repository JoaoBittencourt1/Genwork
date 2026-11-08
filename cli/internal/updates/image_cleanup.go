package updates

import (
	"context"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// genworkImagePrefix is the name every Genwork component image has under the registry, such as genwork-core.
const genworkImagePrefix = "genwork-"

// StoredImage is an image the engine holds.
type StoredImage struct {
	ID         string
	Repository string
	Tag        string
	Size       int64
}

func (image StoredImage) Reference() string {
	return image.Repository + ":" + image.Tag
}

// ImageStore is the engine side of the cleanup (rule 9): the CLI implements it with Docker, tests with a fake.
type ImageStore interface {
	ListImages(ctx context.Context) ([]StoredImage, error)
	// RemoveImage removes one tag; the engine refuses an image a container still uses.
	RemoveImage(ctx context.Context, reference string) error
}

// OutdatedImage is an image the cleanup may remove.
type OutdatedImage struct {
	Reference string `json:"reference"`
	Size      int64  `json:"size"`
}

// CleanupPreview is what the cleanup would remove.
type CleanupPreview struct {
	Images     []OutdatedImage `json:"images"`
	TotalBytes int64           `json:"totalBytes"`
}

// CleanupReport is what the cleanup removed, and what the engine kept because something still used it.
type CleanupReport struct {
	Removed    []OutdatedImage `json:"removed"`
	Skipped    []OutdatedImage `json:"skipped"`
	FreedBytes int64           `json:"freedBytes"`
}

// ImageCleanupService removes the images earlier releases left behind. It touches only Genwork component images from
// the configured registry whose tag is a version other than the installed one: the one action rule 84 allows the
// updater besides installing.
type ImageCleanupService struct {
	store            ImageStore
	registry         string
	installedVersion func() (string, error)
	isUpdating       func() bool
}

func NewImageCleanupService(store ImageStore, registry string, installedVersion func() (string, error), isUpdating func() bool) *ImageCleanupService {
	return &ImageCleanupService{store: store, registry: strings.TrimRight(registry, "/"), installedVersion: installedVersion, isUpdating: isUpdating}
}

func (service *ImageCleanupService) Preview(ctx context.Context) (CleanupPreview, error) {
	outdated, err := service.outdated(ctx)
	if err != nil {
		return CleanupPreview{}, err
	}
	preview := CleanupPreview{Images: make([]OutdatedImage, 0, len(outdated))}
	for _, image := range outdated {
		preview.Images = append(preview.Images, OutdatedImage{Reference: image.Reference(), Size: image.Size})
		preview.TotalBytes += image.Size
	}
	return preview, nil
}

// Remove refuses while an update runs, since an update may still need the previous images to roll back.
func (service *ImageCleanupService) Remove(ctx context.Context) (CleanupReport, error) {
	if service.isUpdating() {
		return CleanupReport{}, newUpdateError(CodeUpdateInProgress, nil)
	}
	outdated, err := service.outdated(ctx)
	if err != nil {
		return CleanupReport{}, err
	}
	report := CleanupReport{Removed: []OutdatedImage{}, Skipped: []OutdatedImage{}}
	for _, image := range outdated {
		entry := OutdatedImage{Reference: image.Reference(), Size: image.Size}
		if err := service.store.RemoveImage(ctx, image.Reference()); err != nil {
			report.Skipped = append(report.Skipped, entry)
			continue
		}
		report.Removed = append(report.Removed, entry)
		report.FreedBytes += image.Size
	}
	return report, nil
}

func (service *ImageCleanupService) outdated(ctx context.Context) ([]StoredImage, error) {
	installed, err := service.installedVersion()
	if err != nil {
		return nil, err
	}
	images, err := service.store.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	var outdated []StoredImage
	for _, image := range images {
		if service.isOutdatedGenworkImage(image, installed) {
			outdated = append(outdated, image)
		}
	}
	return outdated, nil
}

// isOutdatedGenworkImage keeps development tags (":dev"), the installed version, and anything outside the registry.
func (service *ImageCleanupService) isOutdatedGenworkImage(image StoredImage, installed string) bool {
	name, found := strings.CutPrefix(strings.ToLower(image.Repository), strings.ToLower(service.registry)+"/")
	if service.registry == "" || !found || !strings.HasPrefix(name, genworkImagePrefix) || strings.Contains(name, "/") {
		return false
	}
	if _, err := semver.StrictNewVersion(image.Tag); err != nil {
		return false
	}
	return image.Tag != installed
}
