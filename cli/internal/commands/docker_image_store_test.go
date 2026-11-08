package commands

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestDockerImageStoreReadsTagsAndExactSizes(t *testing.T) {
	runner := &fakeRunner{stdout: map[string]string{
		"image ls":      "ghcr.io/acme/genwork-core\t2.2.3\tabc123\n<none>\t<none>\tdangling\nghcr.io/acme/genwork-core\t2.3.0\tdef456\n",
		"image inspect": "sha256:abc123ffff\t734003200\nsha256:def456eeee\t52428800\n",
	}}
	store := newDockerImageStore(runner, io.Discard)

	images, err := store.ListImages(context.Background())

	if err != nil || len(images) != 2 {
		t.Fatalf("images = %+v, err = %v", images, err)
	}
	if images[0].Reference() != "ghcr.io/acme/genwork-core:2.2.3" || images[0].Size != 734003200 || images[1].Size != 52428800 {
		t.Errorf("images = %+v", images)
	}
}

func TestDockerImageStoreRemovesWithoutForcing(t *testing.T) {
	runner := &fakeRunner{}
	store := newDockerImageStore(runner, io.Discard)

	if err := store.RemoveImage(context.Background(), "ghcr.io/acme/genwork-core:2.2.3"); err != nil {
		t.Fatal(err)
	}

	line := runner.runs[0].commandLine()
	if !strings.Contains(line, "image rm ghcr.io/acme/genwork-core:2.2.3") || strings.Contains(line, "-f") {
		t.Errorf("command = %s", line)
	}
}

func TestDockerImageStoreReportsAFailedRemoval(t *testing.T) {
	store := newDockerImageStore(&fakeRunner{failOn: "rm"}, io.Discard)

	if err := store.RemoveImage(context.Background(), "ghcr.io/acme/genwork-core:2.2.3"); err == nil {
		t.Error("a refused removal reported no error")
	}
}
