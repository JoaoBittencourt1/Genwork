package app_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/app"
)

const testUpdaterSecret = "test-updater-secret-0123456789abcdef"

// unreachableUpdaterURL refuses connections, so a test that forgets its fake updater fails instead of reaching one.
const unreachableUpdaterURL = "http://127.0.0.1:1"

type updaterCall struct {
	method        string
	path          string
	authorization string
	body          string
}

type updaterAnswer struct {
	status int
	body   []byte
}

// fakeUpdater answers like the updater container, from status fixtures recorded from its API (rule 26).
type fakeUpdater struct {
	server  *httptest.Server
	mutex   sync.Mutex
	answers map[string]updaterAnswer
	calls   []updaterCall
}

func newFakeUpdater(t *testing.T) *fakeUpdater {
	t.Helper()
	updater := &fakeUpdater{answers: map[string]updaterAnswer{}}
	updater.server = httptest.NewServer(http.HandlerFunc(updater.serve))
	t.Cleanup(updater.server.Close)
	return updater
}

func updaterFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/updater/" + name)
	if err != nil {
		t.Fatalf("read updater fixture %s: %v", name, err)
	}
	return data
}

// answer sets the reply to "METHOD /path" with a status fixture.
func (updater *fakeUpdater) answer(t *testing.T, route string, status int, fixture string) {
	t.Helper()
	updater.mutex.Lock()
	defer updater.mutex.Unlock()
	updater.answers[route] = updaterAnswer{status: status, body: updaterFixture(t, fixture)}
}

func (updater *fakeUpdater) refuse(route string, status int, code string) {
	updater.mutex.Lock()
	defer updater.mutex.Unlock()
	updater.answers[route] = updaterAnswer{status: status, body: []byte(fmt.Sprintf(`{"error":{"code":%q}}`, code))}
}

func (updater *fakeUpdater) recordedCalls() []updaterCall {
	updater.mutex.Lock()
	defer updater.mutex.Unlock()
	return append([]updaterCall{}, updater.calls...)
}

func (updater *fakeUpdater) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	updater.mutex.Lock()
	updater.calls = append(updater.calls, updaterCall{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization"), body: string(body)})
	answer, found := updater.answers[r.Method+" "+r.URL.Path]
	updater.mutex.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(answer.status)
	_, _ = w.Write(answer.body)
}

func withUpdater(updater *fakeUpdater) testAppOption {
	return func(settings *app.Settings) { settings.UpdaterURL = updater.server.URL }
}

type updateStatusBody map[string]any

func (app *testApp) updateStatus(method, path, accessToken string, body any) (response, updateStatusBody) {
	app.t.Helper()
	resp := app.do(request{method: method, path: path, accessToken: accessToken, body: body})
	var decoded updateStatusBody
	_ = json.Unmarshal(resp.body, &decoded)
	return resp, decoded
}
