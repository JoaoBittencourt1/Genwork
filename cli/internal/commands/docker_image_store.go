package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/updates"
)

// dockerImageStore lists and removes images with the Docker CLI, for the updater's cleanup (rule 84).
type dockerImageStore struct {
	runner CommandRunner
	stderr io.Writer
}

func newDockerImageStore(runner CommandRunner, stderr io.Writer) *dockerImageStore {
	return &dockerImageStore{runner: runner, stderr: stderr}
}

// ListImages reads every tagged image with its size in bytes, which only `docker image inspect` reports exactly.
func (store *dockerImageStore) ListImages(ctx context.Context) ([]updates.StoredImage, error) {
	var listed bytes.Buffer
	if err := store.docker(ctx, &listed, "image", "ls", "--format", "{{.Repository}}\t{{.Tag}}\t{{.ID}}"); err != nil {
		return nil, err
	}
	var images []updates.StoredImage
	ids := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(listed.String()), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] == "<none>" || fields[1] == "<none>" {
			continue
		}
		images = append(images, updates.StoredImage{Repository: fields[0], Tag: fields[1], ID: fields[2]})
		ids[fields[2]] = true
	}
	if len(images) == 0 {
		return nil, nil
	}
	sizes, err := store.sizes(ctx, ids)
	if err != nil {
		return nil, err
	}
	for index := range images {
		images[index].Size = sizes[images[index].ID]
	}
	return images, nil
}

func (store *dockerImageStore) sizes(ctx context.Context, ids map[string]bool) (map[string]int64, error) {
	args := []string{"image", "inspect", "--format", "{{.Id}}\t{{.Size}}"}
	for id := range ids {
		args = append(args, id)
	}
	var inspected bytes.Buffer
	if err := store.docker(ctx, &inspected, args...); err != nil {
		return nil, err
	}
	sizes := map[string]int64{}
	for _, line := range strings.Split(strings.TrimSpace(inspected.String()), "\n") {
		id, size, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		bytesUsed, err := strconv.ParseInt(strings.TrimSpace(size), 10, 64)
		if err != nil {
			continue
		}
		for short := range ids {
			if strings.HasPrefix(strings.TrimPrefix(id, "sha256:"), strings.TrimPrefix(short, "sha256:")) {
				sizes[short] = bytesUsed
			}
		}
	}
	return sizes, nil
}

// RemoveImage never forces, so an image a container uses stays and the cleanup reports it as skipped.
func (store *dockerImageStore) RemoveImage(ctx context.Context, reference string) error {
	return store.docker(ctx, io.Discard, "image", "rm", reference)
}

func (store *dockerImageStore) docker(ctx context.Context, stdout io.Writer, args ...string) error {
	if err := store.runner.Run(ctx, RunRequest{Name: "docker", Args: args, Stdout: stdout, Stderr: store.stderr}); err != nil {
		return fmt.Errorf("docker %s: %w", strings.Join(args[:2], " "), err)
	}
	return nil
}
