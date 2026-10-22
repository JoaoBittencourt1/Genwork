package githubsynctest

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Manifest codes the fake GitHub understands; each answers with a recorded response from testdata.
const (
	OrganizationAppCode = "organization-app-code"
	UserAppCode         = "user-app-code"
	ExpiredCode         = "expired-code"
	RejectedCode        = "rejected-code"
	BrokenCode          = "broken-code"
	IncompleteCode      = "incomplete-code"
)

// ClientSecret is the OAuth client secret in the recorded conversions; gentasks must never store it.
const ClientSecret = "fixture-oauth-client-secret-never-stored"

//go:embed testdata
var recordedResponses embed.FS

type recordedResponse struct {
	status int
	file   string
}

var conversionResponses = map[string]recordedResponse{
	OrganizationAppCode: {http.StatusCreated, "testdata/manifest_conversion_organization.json"},
	UserAppCode:         {http.StatusCreated, "testdata/manifest_conversion_user.json"},
	ExpiredCode:         {http.StatusNotFound, "testdata/manifest_conversion_not_found.json"},
	RejectedCode:        {http.StatusUnprocessableEntity, "testdata/manifest_conversion_validation_failed.json"},
	IncompleteCode:      {http.StatusCreated, "testdata/manifest_conversion_without_webhook_secret.json"},
}

// FakeGithub serves the GitHub REST endpoints gentasks calls, so no test reaches the real GitHub (rule 25).
type FakeGithub struct {
	server      *httptest.Server
	mutex       sync.Mutex
	conversions []string
}

func NewFakeGithub(t testing.TB) *FakeGithub {
	t.Helper()
	github := &FakeGithub{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app-manifests/{code}/conversions", github.serveConversion)
	github.server = httptest.NewServer(mux)
	t.Cleanup(github.server.Close)
	return github
}

func (github *FakeGithub) URL() string {
	return github.server.URL
}

// Conversions lists the manifest codes gentasks exchanged, in order.
func (github *FakeGithub) Conversions() []string {
	github.mutex.Lock()
	defer github.mutex.Unlock()
	return append([]string(nil), github.conversions...)
}

func (github *FakeGithub) serveConversion(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	github.mutex.Lock()
	github.conversions = append(github.conversions, code)
	github.mutex.Unlock()

	if r.Header.Get("Accept") != "application/vnd.github+json" {
		http.Error(w, "missing GitHub media type", http.StatusUnsupportedMediaType)
		return
	}
	recorded, found := conversionResponses[code]
	if !found {
		http.Error(w, "unexpected failure", http.StatusInternalServerError)
		return
	}
	body, err := recordedResponses.ReadFile(recorded.file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(recorded.status)
	_, _ = w.Write(body)
}
