package commands

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrWorkspaceNotFound = errors.New("not inside a Genwork repository (no deploy/docker-compose.yml found in this directory or its parents)")

type Workspace struct {
	Root string
}

func (workspace Workspace) ComposeFile() string {
	return filepath.Join(workspace.Root, "deploy", "docker-compose.yml")
}
func (workspace Workspace) StackEnvFile() string {
	return filepath.Join(workspace.Root, "deploy", ".env")
}
func (workspace Workspace) WebDir() string     { return filepath.Join(workspace.Root, "web") }
func (workspace Workspace) WebEnvFile() string { return filepath.Join(workspace.Root, "web", ".env") }

func FindWorkspace(start string) (Workspace, error) {
	directory := start
	for {
		if _, err := os.Stat(filepath.Join(directory, "deploy", "docker-compose.yml")); err == nil {
			return Workspace{Root: directory}, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return Workspace{}, ErrWorkspaceNotFound
		}
		directory = parent
	}
}

// EnsureEnvFiles creates missing .env files from their examples; the stack file gets a fresh JWT signing key.
func (workspace Workspace) EnsureEnvFiles(output io.Writer) error {
	created, err := copyIfMissing(workspace.StackEnvFile(), workspace.StackEnvFile()+".example", func(content string) string {
		return setEnvValue(content, "JWT_SIGNING_KEY", newSigningKeySeed())
	})
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(output, "Created %s with a new JWT signing key\n", workspace.relative(workspace.StackEnvFile()))
	}
	created, err = copyIfMissing(workspace.WebEnvFile(), workspace.WebEnvFile()+".example", func(content string) string { return content })
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(output, "Created %s\n", workspace.relative(workspace.WebEnvFile()))
	}
	return nil
}

func (workspace Workspace) StackEnvValue(name string) string {
	return readEnvValue(workspace.StackEnvFile(), name)
}

func (workspace Workspace) WebEnvValue(name string) string {
	return readEnvValue(workspace.WebEnvFile(), name)
}

func (workspace Workspace) relative(path string) string {
	relative, err := filepath.Rel(workspace.Root, path)
	if err != nil {
		return path
	}
	return relative
}

func copyIfMissing(target, example string, transform func(string) string) (bool, error) {
	if _, err := os.Stat(target); err == nil {
		return false, nil
	}
	content, err := os.ReadFile(example)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", example, err)
	}
	if err := os.WriteFile(target, []byte(transform(string(content))), 0o600); err != nil {
		return false, fmt.Errorf("write %s: %w", target, err)
	}
	return true, nil
}

func setEnvValue(content, name, value string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, name+"=") {
			lines[index] = name + "=" + value
		}
	}
	return strings.Join(lines, "\n")
}

func readEnvValue(path, name string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if value, found := strings.CutPrefix(scanner.Text(), name+"="); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newSigningKeySeed() string {
	seed := make([]byte, 32)
	_, _ = rand.Read(seed)
	return base64.StdEncoding.EncodeToString(seed)
}
