package packaging

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// newRepositoryFixture lays out a repository with everything a bundle must leave out next to what it must carry.
func newRepositoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"VERSION":                        "0.5.0\n",
		"deploy/docker-compose.yml":      "services: {}\n",
		"deploy/docker-compose.dev.yml":  "services: {}\n",
		"deploy/.env.example":            "GATEWAY_PORT=8487\n",
		"deploy/.env":                    "JWT_SIGNING_KEY=secret\n",
		"deploy/Caddyfile":               ":8487 {}\n",
		"deploy/genwork.yaml":            "modules: []\n",
		"deploy/install.sh.tmpl":         installScriptTemplateFixture,
		"modules/gentasks/module.yaml":   "id: gentasks\n",
		"modules/gentasks/go.mod":        "module gentasks\n",
		"modules/gentasks/cmd/main.go":   "package main\n",
		"modules/genmonitor/module.yaml": "id: genmonitor\n",
		"core/cmd/core/main.go":          "package main\n",
		"web/package.json":               "{}\n",
		"web/src/main.tsx":               "export {}\n",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const installScriptTemplateFixture = `#!/bin/sh
RELEASE_URL="{{.ReleaseURL}}"
VERSION="{{.Version}}"
`

// readArchive returns every regular file of a .tar.gz with its content.
func readArchive(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer file.Close()
	decompressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	reader := tar.NewReader(decompressed)
	entries := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatalf("read archive: %v", err)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		entries[header.Name] = string(content)
	}
}

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}
