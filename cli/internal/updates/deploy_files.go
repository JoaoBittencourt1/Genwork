package updates

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/packaging"
)

const maxBundleFileBytes = 4 << 20

// snapshotFiles are the deploy files a swap replaces or regenerates; module manifests are snapshotted as a whole.
var snapshotFiles = []string{"VERSION", "deploy/docker-compose.yml", "deploy/.env.example", "deploy/.env", "deploy/Caddyfile"}

// extractBundle writes only regular files of the bundle layout, so a crafted archive cannot write outside it.
func extractBundle(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	decompressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read the deploy bundle: %w", err)
	}
	reader := tar.NewReader(decompressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read the deploy bundle: %w", err)
		}
		if header.Typeflag != tar.TypeReg || !packaging.IsBundlePath(header.Name) || header.Size > maxBundleFileBytes {
			return fmt.Errorf("the deploy bundle holds an unexpected entry %q", header.Name)
		}
		if err := writeFileFrom(filepath.Join(destination, filepath.FromSlash(header.Name)), io.LimitReader(reader, maxBundleFileBytes), 0o644); err != nil {
			return err
		}
	}
}

// bundlePaths lists the slash-separated files under root that belong to the bundle layout.
func bundlePaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if slashed := filepath.ToSlash(relative); packaging.IsBundlePath(slashed) {
			paths = append(paths, slashed)
		}
		return nil
	})
	return paths, err
}

func moduleManifests(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "modules", "*", "module.yaml"))
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		relative, err := filepath.Rel(root, match)
		if err != nil {
			return nil, err
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	return paths, nil
}

// snapshotDeployFiles copies what a swap changes into snapshot, so a rollback can put it back.
func snapshotDeployFiles(root, snapshot string) error {
	if err := os.RemoveAll(snapshot); err != nil {
		return err
	}
	manifests, err := moduleManifests(root)
	if err != nil {
		return err
	}
	for _, path := range append(append([]string{}, snapshotFiles...), manifests...) {
		if err := copyFile(filepath.Join(root, filepath.FromSlash(path)), filepath.Join(snapshot, filepath.FromSlash(path))); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("keep a copy of %s: %w", path, err)
		}
	}
	return nil
}

// swapDeployFiles copies the staged bundle over the installation; VERSION is written only once the update succeeded.
func swapDeployFiles(stagedRoot, root string) error {
	paths, err := bundlePaths(stagedRoot)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if path == "VERSION" {
			continue
		}
		if err := copyFile(filepath.Join(stagedRoot, filepath.FromSlash(path)), filepath.Join(root, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("install %s: %w", path, err)
		}
	}
	return nil
}

// restoreDeployFiles puts the snapshot back and removes manifests the new bundle added.
func restoreDeployFiles(snapshot, root string) error {
	current, err := moduleManifests(root)
	if err != nil {
		return err
	}
	for _, path := range current {
		if _, err := os.Stat(filepath.Join(snapshot, filepath.FromSlash(path))); errors.Is(err, os.ErrNotExist) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				return err
			}
			_ = os.Remove(filepath.Dir(filepath.Join(root, filepath.FromSlash(path))))
		}
	}
	return filepath.WalkDir(snapshot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(snapshot, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(root, relative))
	})
}

// copyFile rewrites the target in place, keeping its inode: the gateway bind-mounts deploy/Caddyfile.
func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	return writeFileFrom(target, input, info.Mode().Perm())
}

func writeFileFrom(target string, content io.Reader, mode os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	_, err = io.Copy(file, content)
	return err
}

// pruneBackups keeps the newest backups; their names start with a UTC timestamp, so they sort by age.
func pruneBackups(directory string, keep int) error {
	matches, err := filepath.Glob(filepath.Join(directory, "*.dump"))
	if err != nil || len(matches) <= keep {
		return err
	}
	for _, path := range matches[:len(matches)-keep] {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func isNotWritable(err error) bool {
	return errors.Is(err, fs.ErrPermission) || strings.Contains(fmt.Sprint(err), "read-only file system")
}
