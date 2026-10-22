package releases_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
)

func writeAssets(t *testing.T, assets map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, content := range assets {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func sha256Hex(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func TestVerifyChecksumsAcceptsMatchingAssets(t *testing.T) {
	directory := writeAssets(t, map[string]string{
		"bundle.tar.gz":  "bundle",
		"genwork.tar.gz": "binary",
		"checksums.txt":  sha256Hex("bundle") + "  bundle.tar.gz\n" + sha256Hex("binary") + "  genwork.tar.gz\n" + sha256Hex("other") + "  install.sh\n",
	})

	if err := releases.VerifyChecksums(directory, "bundle.tar.gz", "genwork.tar.gz"); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestVerifyChecksumsReportsTheMismatchingAsset(t *testing.T) {
	directory := writeAssets(t, map[string]string{
		"bundle.tar.gz":  "tampered",
		"genwork.tar.gz": "binary",
		"checksums.txt":  sha256Hex("bundle") + "  bundle.tar.gz\n" + sha256Hex("binary") + "  genwork.tar.gz\n",
	})

	err := releases.VerifyChecksums(directory, "genwork.tar.gz", "bundle.tar.gz")

	var mismatch *releases.ChecksumMismatchError
	if !errors.As(err, &mismatch) || mismatch.Asset != "bundle.tar.gz" || releases.ErrorCode(err) != "checksum_mismatch" {
		t.Errorf("error = %v", err)
	}
}

func TestVerifyChecksumsRejectsAnAssetMissingFromTheList(t *testing.T) {
	directory := writeAssets(t, map[string]string{
		"bundle.tar.gz": "bundle",
		"checksums.txt": sha256Hex("other") + "  other.tar.gz\n",
	})

	err := releases.VerifyChecksums(directory, "bundle.tar.gz")

	var mismatch *releases.ChecksumMismatchError
	if !errors.As(err, &mismatch) || mismatch.Asset != "bundle.tar.gz" {
		t.Errorf("error = %v", err)
	}
}

func TestVerifyChecksumsFailsWithoutTheChecksumsFile(t *testing.T) {
	directory := writeAssets(t, map[string]string{"bundle.tar.gz": "bundle"})

	if err := releases.VerifyChecksums(directory, "bundle.tar.gz"); releases.ErrorCode(err) != "checksum_mismatch" {
		t.Errorf("error = %v", err)
	}
}
