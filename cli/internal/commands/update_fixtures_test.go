package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/cli/internal/releases/releasestest"
)

// fakeGateway answers /.well-known/genwork with the version of the last stack start that succeeded.
type fakeGateway struct {
	server  *httptest.Server
	mutex   sync.Mutex
	version string
}

func newFakeGateway(t *testing.T, version string) *fakeGateway {
	t.Helper()
	gateway := &fakeGateway{version: version}
	gateway.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/genwork" {
			http.NotFound(w, r)
			return
		}
		gateway.mutex.Lock()
		defer gateway.mutex.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"version": gateway.version})
	}))
	t.Cleanup(gateway.server.Close)
	return gateway
}

func (gateway *fakeGateway) port(t *testing.T) string {
	t.Helper()
	parsed, err := url.Parse(gateway.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Port()
}

func (gateway *fakeGateway) followStackStarts(run recordedRun, failed bool) {
	if failed || !slices.Contains(run.args, "--wait") || run.version() == "" {
		return
	}
	gateway.mutex.Lock()
	defer gateway.mutex.Unlock()
	gateway.version = run.version()
}

type updateFixture struct {
	workspace  Workspace
	github     *releasestest.FakeGithub
	gateway    *fakeGateway
	runner     *fakeRunner
	executable string
}

// newUpdateFixture is a release installation of 0.5.0 with gentasks, whose release channel and gateway are fakes.
func newUpdateFixture(t *testing.T) *updateFixture {
	t.Helper()
	workspace := newReleaseWorkspace(t)
	installModules(t, workspace, "gentasks")
	github := releasestest.NewFakeGithub(t)
	github.PublishLatestRelease(t)
	gateway := newFakeGateway(t, "0.5.0")
	env := strings.Join([]string{
		"GATEWAY_PORT=" + gateway.port(t), "POSTGRES_USER=genwork", "POSTGRES_DB=genwork",
		"GITHUB_API_URL=" + github.APIURL(), "GENWORK_RELEASE_REPOSITORY=" + releasestest.Repository,
		"JWT_SIGNING_KEY=kept-signing-key", "GENWORK_MODULE_SECRET=kept-module-secret-0123456789",
		"GENWORK_UPDATER_SECRET=kept-updater-secret-0123456789", "GENWORK_HOME=" + workspace.Root,
		"GENTASKS_DB_PASSWORD=kept-gentasks-password", "GENTASKS_DATABASE_URL=postgres://gentasks@postgres/genwork",
	}, "\n") + "\n"
	writeTestFile(t, workspace.StackEnvFile(), env)
	writeTestFile(t, workspace.VersionFile(), "0.5.0\n")
	executable := filepath.Join(workspace.Root, "bin", "genwork")
	writeTestFile(t, executable, "old binary")
	runner := &fakeRunner{
		composeServices: "postgres\ncore-migrate\ncore\ncore-worker\ngentasks-migrate\ngentasks\ngateway\nupdater\n",
		stdout:          map[string]string{"pg_dump": "dump of 0.5.0"},
		onRun:           gateway.followStackStarts,
	}
	return &updateFixture{workspace: workspace, github: github, gateway: gateway, runner: runner, executable: executable}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fixture *updateFixture) runsMatching(fragment string) []recordedRun {
	fixture.runner.mutex.Lock()
	defer fixture.runner.mutex.Unlock()
	var matching []recordedRun
	for _, run := range fixture.runner.runs {
		if strings.Contains(run.commandLine(), fragment) {
			matching = append(matching, run)
		}
	}
	return matching
}
