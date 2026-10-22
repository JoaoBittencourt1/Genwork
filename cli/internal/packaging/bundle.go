// Package packaging builds the release assets that are not binaries: the deploy bundle and install.sh (design §4).
package packaging

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"
)

// bundleFiles plus every modules/<id>/module.yaml is everything a server needs to run a release, laid out like the
// repository (spec "Deploy bundle layout").
var bundleFiles = []string{"VERSION", "deploy/docker-compose.yml", "deploy/.env.example"}

var manifestPathPattern = regexp.MustCompile(`^modules/[a-z][a-z0-9-]{1,39}/module\.yaml$`)

// IsBundlePath tells whether a slash-separated path belongs in a deploy bundle, so an update extracts nothing else.
func IsBundlePath(path string) bool {
	return slices.Contains(bundleFiles, path) || manifestPathPattern.MatchString(path)
}

// WriteDeployBundle writes the bundle as .tar.gz with fixed timestamps, so the same repository always yields the same archive.
func WriteDeployBundle(repositoryRoot, destination string) (err error) {
	manifests, err := filepath.Glob(filepath.Join(repositoryRoot, "modules", "*", "module.yaml"))
	if err != nil {
		return fmt.Errorf("find module manifests: %w", err)
	}
	paths := slices.Clone(bundleFiles)
	for _, manifest := range manifests {
		relative, err := filepath.Rel(repositoryRoot, manifest)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	slices.Sort(paths)

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for _, path := range paths {
		if err := addBundleFile(archive, repositoryRoot, path); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if err := compressed.Close(); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return nil
}

func addBundleFile(archive *tar.Writer, repositoryRoot, path string) error {
	content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(path)))
	if err != nil {
		return fmt.Errorf("read %s for the deploy bundle: %w", path, err)
	}
	header := &tar.Header{Name: path, Mode: 0o644, Size: int64(len(content)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}
	if err := archive.WriteHeader(header); err != nil {
		return fmt.Errorf("add %s to the deploy bundle: %w", path, err)
	}
	if _, err := archive.Write(content); err != nil {
		return fmt.Errorf("add %s to the deploy bundle: %w", path, err)
	}
	return nil
}
