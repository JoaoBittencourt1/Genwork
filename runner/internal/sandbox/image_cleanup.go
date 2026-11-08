package sandbox

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var releaseTagPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// ImagePruner removes sandbox images an earlier runner version pulled, which nothing will use again.
type ImagePruner interface {
	// RemoveOutdatedImages removes the images of the current image's repository tagged with another release version,
	// and returns the bytes freed. An image a container uses is left in place.
	RemoveOutdatedImages(ctx context.Context, currentImage string) (int64, error)
}

// SplitImageReference separates "registry:5000/owner/name:tag" into its repository and tag.
func SplitImageReference(reference string) (string, string) {
	slash := strings.LastIndex(reference, "/")
	colon := strings.LastIndex(reference, ":")
	if colon <= slash {
		return reference, ""
	}
	return reference[:colon], reference[colon+1:]
}

func (sandbox *ContainerSandbox) RemoveOutdatedImages(ctx context.Context, currentImage string) (int64, error) {
	repository, current := SplitImageReference(currentImage)
	var listed []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
		Size     int64    `json:"Size"`
	}
	if _, err := sandbox.engine.do(ctx, "GET", "/images/json", nil, nil, &listed); err != nil {
		return 0, fmt.Errorf("list images: %w", err)
	}
	var freed int64
	for _, image := range listed {
		removedAll := len(image.RepoTags) > 0
		removedAny := false
		for _, tag := range image.RepoTags {
			tagRepository, version := SplitImageReference(tag)
			if tagRepository != repository || version == current || !releaseTagPattern.MatchString(version) {
				removedAll = false
				continue
			}
			if _, err := sandbox.engine.do(ctx, "DELETE", "/images/"+url.PathEscape(tag), nil, nil, nil); err != nil {
				removedAll = false
				continue
			}
			removedAny = true
		}
		if removedAny && removedAll {
			freed += image.Size
		}
	}
	return freed, nil
}
