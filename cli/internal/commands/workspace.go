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
	"slices"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
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
func (workspace Workspace) InstallationFile() string {
	return filepath.Join(workspace.Root, "deploy", "genwork.yaml")
}
func (workspace Workspace) GatewayConfigFile() string {
	return filepath.Join(workspace.Root, "deploy", "Caddyfile")
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

// EnsureEnvFiles creates missing .env files from their examples; the stack file gets a fresh JWT signing key and module secret.
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
	err = workspace.updateStackEnv(output, func(fill envFiller, _ envReader) {
		fill("GENWORK_MODULE_SECRET", newURLSafeSecret())
	})
	if err != nil {
		return err
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

// envFiller sets a value only when the variable is missing or empty; envReader reads the value being written.
type (
	envFiller func(name, value string)
	envReader func(name string) string
)

// EnsureModuleSecrets generates the database password, database URL, and encryption key of every given module that lacks them (design §4).
func (workspace Workspace) EnsureModuleSecrets(output io.Writer, moduleIDs []string) error {
	return workspace.updateStackEnv(output, func(fill envFiller, current envReader) {
		for _, id := range moduleIDs {
			conventions := modules.ConventionsFor(id)
			fill(conventions.PasswordVariable, newURLSafeSecret())
			fill(conventions.EncryptionKeyVariable, newEncryptionKey())
			if database := current("POSTGRES_DB"); database != "" {
				fill(conventions.DatabaseURLVariable, fmt.Sprintf("postgres://%s:%s@postgres:5432/%s?sslmode=disable",
					conventions.DatabaseRole, current(conventions.PasswordVariable), database))
			}
		}
	})
}

// ClearModuleSecrets empties an uninstalled module's credentials, so a reinstall gets fresh ones.
func (workspace Workspace) ClearModuleSecrets(moduleID string) error {
	raw, err := os.ReadFile(workspace.StackEnvFile())
	if err != nil {
		return fmt.Errorf("read %s: %w", workspace.StackEnvFile(), err)
	}
	conventions := modules.ConventionsFor(moduleID)
	content := string(raw)
	for _, variable := range []string{conventions.PasswordVariable, conventions.DatabaseURLVariable, conventions.EncryptionKeyVariable} {
		content = setEnvValue(content, variable, "")
	}
	if err := os.WriteFile(workspace.StackEnvFile(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", workspace.StackEnvFile(), err)
	}
	return nil
}

// updateStackEnv appends keys the example gained since the file was created, then lets fill set missing values.
func (workspace Workspace) updateStackEnv(output io.Writer, update func(fill envFiller, current envReader)) error {
	raw, err := os.ReadFile(workspace.StackEnvFile())
	if err != nil {
		return fmt.Errorf("read %s: %w", workspace.StackEnvFile(), err)
	}
	content, added := appendKeysMissingFrom(string(raw), workspace.StackEnvFile()+".example")
	fill := func(name, value string) {
		if envValueIn(content, name) == "" && value != "" {
			content = upsertEnvValue(content, name, value)
			if !slices.Contains(added, name) {
				added = append(added, name)
			}
		}
	}
	update(fill, func(name string) string { return envValueIn(content, name) })
	if len(added) == 0 {
		return nil
	}
	if err := os.WriteFile(workspace.StackEnvFile(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", workspace.StackEnvFile(), err)
	}
	fmt.Fprintf(output, "Added %s to %s\n", strings.Join(added, ", "), workspace.relative(workspace.StackEnvFile()))
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

// appendKeysMissingFrom adds the example's keys that the file lacks, with their example values, so older .env files keep working.
func appendKeysMissingFrom(content, examplePath string) (string, []string) {
	example, err := os.ReadFile(examplePath)
	if err != nil {
		return content, nil
	}
	var added []string
	for _, line := range strings.Split(string(example), "\n") {
		name, value, found := strings.Cut(line, "=")
		if !found || strings.HasPrefix(name, "#") || envLineExists(content, name) {
			continue
		}
		content = upsertEnvValue(content, name, value)
		added = append(added, name)
	}
	return content, added
}

func upsertEnvValue(content, name, value string) string {
	if envLineExists(content, name) {
		return setEnvValue(content, name, value)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + name + "=" + value + "\n"
}

func envLineExists(content, name string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, name+"=") {
			return true
		}
	}
	return false
}

func envValueIn(content, name string) string {
	for _, line := range strings.Split(content, "\n") {
		if value, found := strings.CutPrefix(line, name+"="); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

// newURLSafeSecret returns 192 random bits that can sit in a URL or a psql string literal without escaping.
func newURLSafeSecret() string {
	secret := make([]byte, 24)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(secret)
}

// newEncryptionKey returns 256 random bits; fill never replaces an existing key, since that would make stored secrets unreadable.
func newEncryptionKey() string {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return base64.RawURLEncoding.EncodeToString(key)
}

func newSigningKeySeed() string {
	seed := make([]byte, 32)
	_, _ = rand.Read(seed)
	return base64.StdEncoding.EncodeToString(seed)
}
