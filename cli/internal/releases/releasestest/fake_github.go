// Package releasestest stubs the GitHub releases API and asset downloads with recorded fixtures (rules 25, 26).
package releasestest

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Repository is the release channel every test configures.
const Repository = "acme/genwork"

// UnroutableAPIURL points at a reserved TEST-NET address, so an accidental real call fails instead of reaching GitHub.
const UnroutableAPIURL = "http://192.0.2.1:9"

//go:embed fixtures/*.json
var fixtures embed.FS

type latestResponse struct {
	status  int
	fixture string
	headers map[string]string
}

// FakeGithub answers GET /repos/{Repository}/releases/latest and serves release assets under /assets/.
type FakeGithub struct {
	server         *httptest.Server
	mutex          sync.Mutex
	latest         latestResponse
	assets         map[string][]byte
	latestRequests int
}

// NewFakeGithub starts with no published release, as a new repository would.
func NewFakeGithub(t testing.TB) *FakeGithub {
	t.Helper()
	github := &FakeGithub{
		latest: latestResponse{status: http.StatusNotFound, fixture: "not_found.json"},
		assets: map[string][]byte{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/"+Repository+"/releases/latest", github.serveLatest)
	mux.HandleFunc("GET /assets/{name}", github.serveAsset)
	github.server = httptest.NewServer(mux)
	t.Cleanup(github.server.Close)
	return github
}

func (github *FakeGithub) APIURL() string {
	return github.server.URL
}

// PublishLatest makes the fixture's release the latest one; its asset URLs point at this server.
func (github *FakeGithub) PublishLatest(fixture string) {
	github.RespondToLatest(http.StatusOK, fixture, nil)
}

func (github *FakeGithub) RespondToLatest(status int, fixture string, headers map[string]string) {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	github.latest = latestResponse{status: status, fixture: fixture, headers: headers}
}

// RateLimit answers like GitHub once an address used up its anonymous quota.
func (github *FakeGithub) RateLimit() {
	github.RespondToLatest(http.StatusForbidden, "rate_limited.json", map[string]string{"X-RateLimit-Remaining": "0"})
}

func (github *FakeGithub) SetAsset(name string, content []byte) {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	github.assets[name] = content
}

func (github *FakeGithub) LatestRequests() int {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	return github.latestRequests
}

func (github *FakeGithub) serveLatest(w http.ResponseWriter, _ *http.Request) {
	github.mutex.Lock()
	github.latestRequests++
	latest := github.latest
	github.mutex.Unlock()
	for name, value := range latest.headers {
		w.Header().Set(name, value)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(latest.status)
	body := strings.ReplaceAll(string(Fixture(latest.fixture)), "{{assets}}", github.server.URL+"/assets")
	_, _ = w.Write([]byte(body))
}

func (github *FakeGithub) serveAsset(w http.ResponseWriter, r *http.Request) {
	github.mutex.Lock()
	content, found := github.assets[r.PathValue("name")]
	github.mutex.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(content)
}

func Fixture(name string) []byte {
	data, err := fixtures.ReadFile("fixtures/" + name)
	if err != nil {
		panic(fmt.Sprintf("unknown GitHub fixture %q: %v", name, err))
	}
	return data
}
