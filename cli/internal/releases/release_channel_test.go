package releases_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

func newChannel(apiURL string) *releases.GithubReleaseChannel {
	return releases.NewGithubReleaseChannel(releases.GithubReleaseChannelConfig{
		APIURL: apiURL, Repository: releasestest.Repository, UserAgent: "genwork/0.5.0",
	})
}

func TestFindLatestReadsThePublishedRelease(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.PublishLatest("latest_release.json")

	release, err := newChannel(github.APIURL()).FindLatest(context.Background())

	if err != nil {
		t.Fatalf("find latest: %v", err)
	}
	if release.Version != "0.5.2" || release.NotesURL != "https://github.com/acme/genwork/releases/tag/v0.5.2" ||
		!release.PublishedAt.Equal(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("release = %+v", release)
	}
	if release.Assets["checksums.txt"] != github.APIURL()+"/assets/checksums.txt" || len(release.Assets) != 7 {
		t.Errorf("assets = %v", release.Assets)
	}
}

func TestFindLatestWithoutAnyRelease(t *testing.T) {
	github := releasestest.NewFakeGithub(t)

	_, err := newChannel(github.APIURL()).FindLatest(context.Background())

	if !errors.Is(err, releases.ErrNoPublishedRelease) {
		t.Errorf("error = %v", err)
	}
}

func TestFindLatestWhenRateLimited(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.RateLimit()

	_, err := newChannel(github.APIURL()).FindLatest(context.Background())

	if releases.ErrorCode(err) != "release_channel_rate_limited" {
		t.Errorf("error = %v, code = %q", err, releases.ErrorCode(err))
	}
}

func TestFindLatestWhenTheChannelIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := newChannel(releasestest.UnroutableAPIURL).FindLatest(ctx)

	if releases.ErrorCode(err) != "release_channel_unreachable" {
		t.Errorf("error = %v, code = %q", err, releases.ErrorCode(err))
	}
}

func TestFindLatestRejectsATagThatIsNotAVersion(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.PublishLatest("unversioned_release.json")

	_, err := newChannel(github.APIURL()).FindLatest(context.Background())

	if releases.ErrorCode(err) != "release_channel_unreachable" || !strings.Contains(err.Error(), "nightly") {
		t.Errorf("error = %v", err)
	}
}

func TestDownloadWritesTheAsset(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.PublishLatest("latest_release.json")
	github.SetAsset("checksums.txt", []byte("content"))
	channel := newChannel(github.APIURL())
	release, err := channel.FindLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "checksums.txt")

	if err := channel.Download(context.Background(), release, "checksums.txt", destination); err != nil {
		t.Fatalf("download: %v", err)
	}

	if content, _ := os.ReadFile(destination); string(content) != "content" {
		t.Errorf("content = %q", content)
	}
}

func TestDownloadFailsForAMissingOrUnpublishedAsset(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.PublishLatest("latest_release.json")
	channel := newChannel(github.APIURL())
	release, err := channel.FindLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()

	for _, name := range []string{"install.sh", "genwork_0.5.2_plan9_amd64.tar.gz"} {
		err := channel.Download(context.Background(), release, name, filepath.Join(directory, name))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: error = %v", name, err)
		}
		if _, statErr := os.Stat(filepath.Join(directory, name)); statErr == nil {
			t.Errorf("%s: a failed download must leave no file behind", name)
		}
	}
}

func TestDownloadRefusesAnAssetOverTheSizeLimit(t *testing.T) {
	github := releasestest.NewFakeGithub(t)
	github.PublishLatest("latest_release.json")
	github.SetAsset("checksums.txt", []byte(strings.Repeat("x", 2048)))
	channel := releases.NewGithubReleaseChannel(releases.GithubReleaseChannelConfig{
		APIURL: github.APIURL(), Repository: releasestest.Repository, UserAgent: "genwork/0.5.0", MaxAssetBytes: 1024,
	})
	release, err := channel.FindLatest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "checksums.txt")

	err = channel.Download(context.Background(), release, "checksums.txt", destination)

	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("error = %v", err)
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		t.Error("an oversized download must leave no file behind")
	}
}

func TestReleaseIsNewerThan(t *testing.T) {
	release := releases.Release{Version: "0.5.2"}
	if !release.IsNewerThan("0.5.0") || release.IsNewerThan("0.5.2") || release.IsNewerThan("0.6.0") || !release.IsNewerThan("0.0.0-dev") {
		t.Error("IsNewerThan compares semantic versions")
	}
}
