package releasestest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/packaging"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
)

// LatestVersion is the release latest_release.json describes.
const LatestVersion = "0.5.2"

// BundleFiles is the deploy bundle of LatestVersion, laid out like the repository.
func BundleFiles() map[string]string {
	return map[string]string{
		"VERSION":                       LatestVersion + "\n",
		"deploy/docker-compose.yml":     "name: genwork\nservices: {}\n# " + LatestVersion + "\n",
		"deploy/.env.example":           "GATEWAY_PORT=8487\nPOSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nGENWORK_MODULE_SECRET=\nNEW_RELEASE_SECRET=\nNEW_RELEASE_SETTING=enabled\n",
		"modules/gentasks/module.yaml":  "id: gentasks\ncore: \">=1.0.0\"\napiPrefix: /api/gentasks/v1\n",
		"modules/reporting/module.yaml": "id: reporting\ncore: \">=1.0.0\"\napiPrefix: /api/reporting/v1\n",
	}
}

// ExecutableContent is what the genwork binary of LatestVersion holds in these tests.
const ExecutableContent = "#!/bin/sh\necho " + LatestVersion + "\n"

// PublishLatestRelease makes LatestVersion the latest release with a real bundle, CLI archive for this machine, and checksums.
func (github *FakeGithub) PublishLatestRelease(t testing.TB) {
	t.Helper()
	github.PublishLatest("latest_release.json")
	assets := map[string][]byte{
		BundleAsset(): buildBundle(t, BundleFiles()),
		ExecutableAsset(runtime.GOOS, runtime.GOARCH): buildArchive(t, map[string]string{"genwork": ExecutableContent}),
	}
	var checksums strings.Builder
	for _, name := range slices.Sorted(maps.Keys(assets)) {
		digest := sha256.Sum256(assets[name])
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(digest[:]), name)
		github.SetAsset(name, assets[name])
	}
	github.SetAsset("checksums.txt", []byte(checksums.String()))
}

// TamperAsset replaces an asset after its checksum was recorded, as a corrupted download would.
func (github *FakeGithub) TamperAsset(name string) {
	github.SetAsset(name, []byte("tampered"))
}

func ExecutableAsset(goos, goarch string) string {
	return releases.ExecutableAsset(LatestVersion, goos, goarch)
}

func BundleAsset() string {
	return releases.BundleAsset(LatestVersion)
}

func buildBundle(t testing.TB, files map[string]string) []byte {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := packaging.WriteDeployBundle(root, destination); err != nil {
		t.Fatalf("build bundle: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func buildArchive(t testing.TB, files map[string]string) []byte {
	t.Helper()
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	writer := tar.NewWriter(compressed)
	for name, content := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}
