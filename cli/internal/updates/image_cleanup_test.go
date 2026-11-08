package updates

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// fakeImageStore plays the engine: inUse lists the references a container still uses, which it refuses to remove.
type fakeImageStore struct {
	images  []StoredImage
	inUse   map[string]bool
	removed []string
}

func (store *fakeImageStore) ListImages(context.Context) ([]StoredImage, error) {
	return store.images, nil
}

func (store *fakeImageStore) RemoveImage(_ context.Context, reference string) error {
	if store.inUse[reference] {
		return errors.New("image is being used by a container")
	}
	store.removed = append(store.removed, reference)
	return nil
}

func installedImagesFixture() *fakeImageStore {
	return &fakeImageStore{images: []StoredImage{
		{ID: "a", Repository: "ghcr.io/acme/genwork-core", Tag: "2.3.0", Size: 100},
		{ID: "b", Repository: "ghcr.io/acme/genwork-core", Tag: "2.2.3", Size: 90},
		{ID: "c", Repository: "ghcr.io/Acme/genwork-gateway", Tag: "2.2.2", Size: 50},
		{ID: "d", Repository: "genwork-core", Tag: "dev", Size: 70},
		{ID: "e", Repository: "ghcr.io/acme/genwork-core", Tag: "dev", Size: 70},
		{ID: "f", Repository: "postgres", Tag: "17-alpine", Size: 300},
		{ID: "g", Repository: "ghcr.io/acme/other-app", Tag: "1.0.0", Size: 40},
		{ID: "h", Repository: "ghcr.io/acme/genwork-sandbox/extra", Tag: "1.0.0", Size: 40},
	}}
}

func newCleanupFixture(store *fakeImageStore, updating bool) *ImageCleanupService {
	return NewImageCleanupService(store, "ghcr.io/acme/", func() (string, error) { return "2.3.0", nil }, func() bool { return updating })
}

func TestCleanupListsOnlyGenworkImagesOfOtherVersions(t *testing.T) {
	preview, err := newCleanupFixture(installedImagesFixture(), false).Preview(context.Background())

	if err != nil || len(preview.Images) != 2 || preview.TotalBytes != 140 {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	if preview.Images[0].Reference != "ghcr.io/acme/genwork-core:2.2.3" || preview.Images[1].Reference != "ghcr.io/Acme/genwork-gateway:2.2.2" {
		t.Errorf("images = %+v", preview.Images)
	}
}

func TestCleanupRemovesOutdatedImagesAndSkipsTheOnesInUse(t *testing.T) {
	store := installedImagesFixture()
	store.inUse = map[string]bool{"ghcr.io/Acme/genwork-gateway:2.2.2": true}

	report, err := newCleanupFixture(store, false).Remove(context.Background())

	if err != nil || len(report.Removed) != 1 || len(report.Skipped) != 1 || report.FreedBytes != 90 {
		t.Fatalf("report = %+v, err = %v", report, err)
	}
	if len(store.removed) != 1 || store.removed[0] != "ghcr.io/acme/genwork-core:2.2.3" {
		t.Errorf("removed = %v", store.removed)
	}
}

func TestCleanupWaitsForAnUpdateToFinish(t *testing.T) {
	store := installedImagesFixture()

	_, err := newCleanupFixture(store, true).Remove(context.Background())

	if ErrorCode(err) != CodeUpdateInProgress || len(store.removed) != 0 {
		t.Errorf("err = %v, removed = %v", err, store.removed)
	}
}

func TestCleanupWithoutARegistryTouchesNothing(t *testing.T) {
	service := NewImageCleanupService(installedImagesFixture(), "", func() (string, error) { return "2.3.0", nil }, func() bool { return false })

	preview, err := service.Preview(context.Background())

	if err != nil || len(preview.Images) != 0 {
		t.Errorf("preview = %+v, err = %v", preview, err)
	}
}

func TestCleanupRoutesNeedTheSecret(t *testing.T) {
	fixture := newUpdaterFixture(t)

	unauthorized, _ := fixture.call(t, http.MethodGet, "/v1/cleanup", "", "")
	preview, previewBody := fixture.call(t, http.MethodGet, "/v1/cleanup", updaterSecret, "")
	removal, removalBody := fixture.call(t, http.MethodPost, "/v1/cleanup", updaterSecret, "")

	if unauthorized != http.StatusUnauthorized || preview != http.StatusOK || removal != http.StatusOK {
		t.Fatalf("statuses = %d %d %d", unauthorized, preview, removal)
	}
	if previewBody["totalBytes"] != float64(0) || removalBody["freedBytes"] != float64(0) {
		t.Errorf("preview = %v, removal = %v", previewBody, removalBody)
	}
}
