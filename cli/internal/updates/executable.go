package updates

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
)

const maxExecutableBytes = 256 << 20

// ErrExecutableNotWritable leaves the binary as it was; the caller explains how to replace it by hand.
var ErrExecutableNotWritable = errors.New("the directory of the genwork binary is not writable")

// ReplaceExecutable installs the release's genwork binary for goos/goarch over executable, verified against
// checksums.txt and renamed into place, so the old binary keeps working until the new one is complete.
func ReplaceExecutable(ctx context.Context, channel releases.ReleaseChannel, release releases.Release, executable, goos, goarch string) error {
	staging, err := os.MkdirTemp("", "genwork-executable-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	asset := releases.ExecutableAsset(release.Version, goos, goarch)
	for _, name := range []string{releases.ChecksumsAsset, asset} {
		if err := channel.Download(ctx, release, name, filepath.Join(staging, name)); err != nil {
			return err
		}
	}
	if err := releases.VerifyChecksums(staging, asset); err != nil {
		return err
	}
	return installExecutable(filepath.Join(staging, asset), executable)
}

func installExecutable(archivePath, executable string) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(executable), ".genwork-*")
	if err != nil {
		if isNotWritable(err) {
			return ErrExecutableNotWritable
		}
		return err
	}
	defer func() {
		if err != nil {
			temporary.Close()
			os.Remove(temporary.Name())
		}
	}()
	if err := extractExecutable(archivePath, temporary); err != nil {
		return err
	}
	if err := temporary.Chmod(0o755); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), executable); err != nil {
		if isNotWritable(err) {
			return ErrExecutableNotWritable
		}
		return err
	}
	return nil
}

func extractExecutable(archivePath string, destination io.Writer) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	decompressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(archivePath), err)
	}
	reader := tar.NewReader(decompressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s holds no genwork binary", filepath.Base(archivePath))
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(archivePath), err)
		}
		if header.Typeflag == tar.TypeReg && header.Name == "genwork" {
			_, err := io.Copy(destination, io.LimitReader(reader, maxExecutableBytes))
			return err
		}
	}
}
