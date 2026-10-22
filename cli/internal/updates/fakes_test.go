package updates

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases"
	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

const installedVersion = "0.5.0"

var fixedNow = time.Date(2026, 10, 22, 13, 55, 0, 0, time.UTC)

// installationFiles is a release installation of installedVersion with its configuration and module choice.
var installationFiles = map[string]string{
	"VERSION":                      installedVersion + "\n",
	"deploy/docker-compose.yml":    "name: genwork\nservices: {}\n# " + installedVersion + "\n",
	"deploy/.env.example":          "GATEWAY_PORT=8487\nPOSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nGENWORK_MODULE_SECRET=\n",
	"deploy/.env":                  "GATEWAY_PORT=8487\nPOSTGRES_USER=genwork\nPOSTGRES_DB=genwork\nGENWORK_MODULE_SECRET=kept-secret\n",
	"deploy/Caddyfile":             ":8487 {}\n",
	"deploy/genwork.yaml":          "modules:\n  - gentasks\n",
	"modules/gentasks/module.yaml": "id: gentasks\ncore: \">=1.0.0\"\napiPrefix: /api/gentasks/v1\n",
}

// fakeInstallation records each operation the engine asks for; failures name the operation to fail, e.g. "start 0.5.2".
type fakeInstallation struct {
	root         string
	repository   bool
	failures     map[string]bool
	mutex        sync.Mutex
	calls        []string
	running      string
	stuckVersion string
	restoredDump string
}

func newFakeInstallation(t *testing.T) *fakeInstallation {
	t.Helper()
	root := t.TempDir()
	for path, content := range installationFiles {
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), content)
	}
	return &fakeInstallation{root: root, failures: map[string]bool{}, running: installedVersion}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (installation *fakeInstallation) record(call string) error {
	installation.mutex.Lock()
	defer installation.mutex.Unlock()
	installation.calls = append(installation.calls, call)
	if installation.failures[call] {
		return errors.New(call + " failed")
	}
	return nil
}

func (installation *fakeInstallation) recordedCalls() []string {
	installation.mutex.Lock()
	defer installation.mutex.Unlock()
	return append([]string{}, installation.calls...)
}

func (installation *fakeInstallation) Root() string               { return installation.root }
func (installation *fakeInstallation) IsRepositoryCheckout() bool { return installation.repository }

func (installation *fakeInstallation) PullImages(_ context.Context, version, stagedRoot string) error {
	if _, err := os.Stat(filepath.Join(stagedRoot, "deploy", "docker-compose.yml")); err != nil {
		return err
	}
	return installation.record("pull " + version)
}

func (installation *fakeInstallation) DumpDatabase(_ context.Context, destination io.Writer) error {
	if err := installation.record("dump"); err != nil {
		return err
	}
	_, err := io.WriteString(destination, "dump of "+installedVersion)
	return err
}

func (installation *fakeInstallation) RefreshConfiguration(context.Context) error {
	return installation.record("refresh")
}

func (installation *fakeInstallation) StartStack(_ context.Context, version string) error {
	if err := installation.record("start " + version); err != nil {
		return err
	}
	installation.mutex.Lock()
	defer installation.mutex.Unlock()
	installation.running = version
	return nil
}

func (installation *fakeInstallation) StopServicesExceptDatabase(_ context.Context, version string) error {
	return installation.record("stop " + version)
}

func (installation *fakeInstallation) RestoreDatabase(_ context.Context, dump io.Reader) error {
	if err := installation.record("restore"); err != nil {
		return err
	}
	content, err := io.ReadAll(dump)
	installation.restoredDump = string(content)
	return err
}

func (installation *fakeInstallation) ReportedVersion(context.Context) (string, error) {
	installation.mutex.Lock()
	defer installation.mutex.Unlock()
	if installation.stuckVersion != "" {
		return installation.stuckVersion, nil
	}
	return installation.running, nil
}

func (installation *fakeInstallation) readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(installation.root, filepath.FromSlash(path)))
	if err != nil {
		return ""
	}
	return string(content)
}

type runnerFixture struct {
	installation *fakeInstallation
	github       *releasestest.FakeGithub
	progress     *bytes.Buffer
	runner       *UpdateRunner
}

func newRunnerFixture(t *testing.T) *runnerFixture {
	t.Helper()
	fixture := &runnerFixture{installation: newFakeInstallation(t), github: releasestest.NewFakeGithub(t), progress: &bytes.Buffer{}}
	fixture.github.PublishLatestRelease(t)
	fixture.runner = fixture.newRunner()
	return fixture
}

func (fixture *runnerFixture) newRunner() *UpdateRunner {
	return NewUpdateRunner(UpdateRunnerConfig{
		Installation: fixture.installation,
		Channel: releases.NewGithubReleaseChannel(releases.GithubReleaseChannelConfig{
			APIURL: fixture.github.APIURL(), Repository: releasestest.Repository, UserAgent: "genwork/test",
		}),
		Now:           func() time.Time { return fixedNow },
		Progress:      fixture.progress,
		VerifyTimeout: 300 * time.Millisecond,
		PollInterval:  10 * time.Millisecond,
	})
}

// update checks for the latest release, then applies it as `genwork update` does.
func (fixture *runnerFixture) update(t *testing.T) UpdateStatus {
	t.Helper()
	if _, err := fixture.runner.Check(context.Background()); err != nil {
		t.Fatalf("check: %v", err)
	}
	running, err := fixture.runner.Begin(releasestest.LatestVersion)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	return running.Run(context.Background())
}

func (fixture *runnerFixture) expectUntouched(t *testing.T) {
	t.Helper()
	for path, content := range installationFiles {
		if got := fixture.installation.readFile(t, path); got != content {
			t.Errorf("%s changed to %q", path, got)
		}
	}
	if fixture.installation.readFile(t, "modules/reporting/module.yaml") != "" {
		t.Error("the new release's manifests must not be installed")
	}
}

func backupNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "backups"))
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".dump") {
			names = append(names, entry.Name())
		}
	}
	return names
}
