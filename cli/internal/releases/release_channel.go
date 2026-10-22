// Package releases reads the platform's published releases and downloads their assets (design §6).
package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	apiTimeout           = 15 * time.Second
	downloadTimeout      = 10 * time.Minute
	defaultMaxAssetBytes = 256 << 20
	maxAPIResponseBytes  = 1 << 20
	githubAPIVersion     = "2022-11-28"
)

// Release is the latest published platform release: drafts and pre-releases never count.
type Release struct {
	Version     string
	NotesURL    string
	PublishedAt time.Time
	// Assets maps each asset name to its download URL.
	Assets map[string]string
}

func (release Release) IsNewerThan(installedVersion string) bool {
	latest, latestErr := semver.StrictNewVersion(release.Version)
	installed, installedErr := semver.StrictNewVersion(installedVersion)
	return latestErr == nil && installedErr == nil && latest.GreaterThan(installed)
}

// ReleaseChannel is where releases come from; the update engine and `genwork install` depend on it, never on GitHub (rule 9).
type ReleaseChannel interface {
	FindLatest(ctx context.Context) (Release, error)
	Download(ctx context.Context, release Release, assetName, destination string) error
}

type GithubReleaseChannelConfig struct {
	// APIURL is GITHUB_API_URL and Repository is GENWORK_RELEASE_REPOSITORY, both from deploy/.env (rule 1).
	APIURL     string
	Repository string
	UserAgent  string
	// MaxAssetBytes bounds every download; zero means 256 MiB.
	MaxAssetBytes int64
}

type GithubReleaseChannel struct {
	config    GithubReleaseChannelConfig
	apiClient *http.Client
	assets    *http.Client
}

func NewGithubReleaseChannel(config GithubReleaseChannelConfig) *GithubReleaseChannel {
	if config.MaxAssetBytes == 0 {
		config.MaxAssetBytes = defaultMaxAssetBytes
	}
	return &GithubReleaseChannel{
		config:    config,
		apiClient: &http.Client{Timeout: apiTimeout},
		assets:    &http.Client{Timeout: downloadTimeout},
	}
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (channel *GithubReleaseChannel) FindLatest(ctx context.Context) (Release, error) {
	url := strings.TrimRight(channel.config.APIURL, "/") + "/repos/" + channel.config.Repository + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, newChannelError(CodeChannelUnreachable, err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("User-Agent", channel.config.UserAgent)
	response, err := channel.apiClient.Do(request)
	if err != nil {
		return Release{}, newChannelError(CodeChannelUnreachable, err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return Release{}, ErrNoPublishedRelease
	case response.StatusCode == http.StatusTooManyRequests ||
		(response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0"):
		return Release{}, newChannelError(CodeChannelRateLimited, fmt.Errorf("GitHub API rate limit reached for %s", channel.config.Repository))
	case response.StatusCode != http.StatusOK:
		return Release{}, newChannelError(CodeChannelUnreachable, fmt.Errorf("GitHub API answered %s", response.Status))
	}
	var body githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, maxAPIResponseBytes)).Decode(&body); err != nil {
		return Release{}, newChannelError(CodeChannelUnreachable, fmt.Errorf("decode the latest release: %w", err))
	}
	return body.toRelease()
}

func (body githubRelease) toRelease() (Release, error) {
	version := strings.TrimPrefix(body.TagName, "v")
	if _, err := semver.StrictNewVersion(version); err != nil {
		return Release{}, newChannelError(CodeChannelUnreachable, fmt.Errorf("the latest release tag %q is not v<semantic version>", body.TagName))
	}
	release := Release{Version: version, NotesURL: body.HTMLURL, PublishedAt: body.PublishedAt, Assets: map[string]string{}}
	for _, asset := range body.Assets {
		release.Assets[asset.Name] = asset.BrowserDownloadURL
	}
	return release, nil
}

// Download writes to a temporary file next to destination and renames it, so a failed download leaves nothing behind.
func (channel *GithubReleaseChannel) Download(ctx context.Context, release Release, assetName, destination string) (err error) {
	url, found := release.Assets[assetName]
	if !found {
		return fmt.Errorf("release v%s has no asset %s", release.Version, assetName)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	request.Header.Set("User-Agent", channel.config.UserAgent)
	response, err := channel.assets.Do(request)
	if err != nil {
		return newChannelError(CodeChannelUnreachable, fmt.Errorf("download %s: %w", assetName, err))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: the server answered %s", assetName, response.Status)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	defer func() {
		if err != nil {
			temporary.Close()
			os.Remove(temporary.Name())
		}
	}()
	written, err := io.Copy(temporary, io.LimitReader(response.Body, channel.config.MaxAssetBytes+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	if written > channel.config.MaxAssetBytes {
		return fmt.Errorf("download %s: the asset is over the %d-byte limit", assetName, channel.config.MaxAssetBytes)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	if err := os.Rename(temporary.Name(), destination); err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	return nil
}

var _ ReleaseChannel = (*GithubReleaseChannel)(nil)

// ErrNoPublishedRelease means the channel has no release yet, which is not a failure of the channel.
var ErrNoPublishedRelease = errors.New("the release channel has no published release")
