package releases

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ChecksumsAsset lists the SHA-256 of every other asset of a release.
const ChecksumsAsset = "checksums.txt"

// VerifyChecksums checks each named file of directory against directory/checksums.txt before anything is extracted.
func VerifyChecksums(directory string, assetNames ...string) error {
	expected, err := readChecksums(filepath.Join(directory, ChecksumsAsset))
	if err != nil {
		return &ChecksumMismatchError{Asset: ChecksumsAsset, Reason: "cannot be read: " + err.Error()}
	}
	for _, name := range assetNames {
		want, listed := expected[name]
		if !listed {
			return &ChecksumMismatchError{Asset: name, Reason: "is not listed in " + ChecksumsAsset}
		}
		actual, err := sha256Of(filepath.Join(directory, name))
		if err != nil {
			return &ChecksumMismatchError{Asset: name, Reason: "cannot be read: " + err.Error()}
		}
		if actual != want {
			return &ChecksumMismatchError{Asset: name, Reason: "does not match " + ChecksumsAsset}
		}
	}
	return nil
}

// readChecksums parses sha256sum output: "<hex digest>  <file name>" per line.
func readChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	checksums := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		digest, name, found := strings.Cut(strings.TrimSpace(scanner.Text()), "  ")
		if found {
			checksums[strings.TrimPrefix(name, "*")] = strings.ToLower(digest)
		}
	}
	return checksums, scanner.Err()
}

func sha256Of(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
