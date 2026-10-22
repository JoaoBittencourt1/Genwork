package commands

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/modules"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/versionfile"
)

var ErrWorkspaceNotFound = errors.New("not inside a Genwork installation or repository (no deploy/docker-compose.yml found in this directory or its parents)")

// genworkHomeVariable names the installation directory, so a server's operator can run genwork from anywhere (design §5).
const genworkHomeVariable = "GENWORK_HOME"

type Workspace struct {
	Root string
}

// WorkspaceKind tells a clone of the repository, which builds from source, from a release installation (design §5).
type WorkspaceKind string

const (
	WorkspaceRepository WorkspaceKind = "repository"
	WorkspaceRelease    WorkspaceKind = "release"
)

// Kind is decided by the web sources, which a deploy bundle never carries.
func (workspace Workspace) Kind() WorkspaceKind {
	if _, err := os.Stat(filepath.Join(workspace.WebDir(), "package.json")); err == nil {
		return WorkspaceRepository
	}
	return WorkspaceRelease
}

func (workspace Workspace) VersionFile() string {
	return filepath.Join(workspace.Root, "VERSION")
}
func (workspace Workspace) ComposeFile() string {
	return filepath.Join(workspace.Root, "deploy", "docker-compose.yml")
}
func (workspace Workspace) DevComposeFile() string {
	return filepath.Join(workspace.Root, "deploy", "docker-compose.dev.yml")
}

// ComposeFiles are the -f arguments of every compose call: the published images, plus local builds in a repository checkout.
func (workspace Workspace) ComposeFiles() []string {
	if workspace.Kind() == WorkspaceRepository {
		return []string{"-f", workspace.ComposeFile(), "-f", workspace.DevComposeFile()}
	}
	return []string{"-f", workspace.ComposeFile()}
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

// FindWorkspace uses home, the value of GENWORK_HOME, when set; otherwise it walks up from start.
func FindWorkspace(start, home string) (Workspace, error) {
	if home != "" {
		root, err := filepath.Abs(home)
		if err != nil {
			return Workspace{}, fmt.Errorf("%s=%s: %w", genworkHomeVariable, home, err)
		}
		if _, err := os.Stat(filepath.Join(root, "deploy", "docker-compose.yml")); err != nil {
			return Workspace{}, fmt.Errorf("%w: %s=%s has no deploy/docker-compose.yml", ErrWorkspaceNotFound, genworkHomeVariable, root)
		}
		return Workspace{Root: root}, nil
	}
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

// EnsureEnvFiles creates missing .env files from their examples; the stack file gets a fresh JWT signing key, a random
// value for every empty *_SECRET key, and the installation's directory, which the updater mounts (design §8).
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
		for _, name := range workspace.exampleSecretNames() {
			fill(name, newURLSafeSecret())
		}
	})
	if err != nil {
		return err
	}
	if err := workspace.recordHome(output); err != nil {
		return err
	}
	if workspace.Kind() == WorkspaceRelease {
		return nil
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

// exampleSecretNames are the example's *_SECRET keys: an update can add one, and the CLI of the previous release fills it
// without knowing its name.
func (workspace Workspace) exampleSecretNames() []string {
	example, err := os.ReadFile(workspace.StackEnvFile() + ".example")
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(example), "\n") {
		if name, _, found := strings.Cut(line, "="); found && !strings.HasPrefix(name, "#") && strings.HasSuffix(name, "_SECRET") {
			names = append(names, name)
		}
	}
	return names
}

// recordHome keeps GENWORK_HOME on the workspace's absolute path, even after the installation directory moved.
func (workspace Workspace) recordHome(output io.Writer) error {
	if workspace.StackEnvValue(genworkHomeVariable) == workspace.Root {
		return nil
	}
	if err := workspace.SetStackEnvValues(map[string]string{genworkHomeVariable: workspace.Root}); err != nil {
		return err
	}
	fmt.Fprintf(output, "Set %s to %s in %s\n", genworkHomeVariable, workspace.Root, workspace.relative(workspace.StackEnvFile()))
	return nil
}

// SetStackEnvValues replaces or appends each variable in deploy/.env, keeping every other line.
func (workspace Workspace) SetStackEnvValues(values map[string]string) error {
	raw, err := os.ReadFile(workspace.StackEnvFile())
	if err != nil {
		return fmt.Errorf("read %s: %w", workspace.StackEnvFile(), err)
	}
	content := string(raw)
	names := slices.Sorted(maps.Keys(values))
	for _, name := range names {
		content = upsertEnvValue(content, name, values[name])
	}
	if err := os.WriteFile(workspace.StackEnvFile(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", workspace.StackEnvFile(), err)
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
	conventions := modules.ConventionsFor(moduleID)
	return workspace.SetStackEnvValues(map[string]string{
		conventions.PasswordVariable: "", conventions.DatabaseURLVariable: "", conventions.EncryptionKeyVariable: "",
	})
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

// Version is the platform version of the workspace's files (rule 50).
func (workspace Workspace) Version() (string, error) {
	return versionfile.Read(workspace.VersionFile())
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
