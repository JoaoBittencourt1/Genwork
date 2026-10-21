// Package apitest serves recorded Genwork API responses, so no test reaches a real instance (rule 25).
package apitest

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

//go:embed fixtures/*.json
var fixtures embed.FS

func Fixture(t testing.TB, name string) []byte {
	t.Helper()
	content, err := fixtures.ReadFile("fixtures/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}

type RecordedRequest struct {
	Method        string
	Path          string
	Query         url.Values
	Authorization string
	ContentType   string
	Body          json.RawMessage
}

// DecodedBody answers the request body as generic JSON, or nil when the request had none.
func (request RecordedRequest) DecodedBody(t testing.TB) any {
	t.Helper()
	if len(request.Body) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(request.Body, &decoded); err != nil {
		t.Fatalf("decode request body %s: %v", request.Body, err)
	}
	return decoded
}

// FakeAPI routes "METHOD /path" to a recorded response or a handler; anything else answers 404 not_found.
type FakeAPI struct {
	server   *httptest.Server
	mutex    sync.Mutex
	handlers map[string]http.HandlerFunc
	requests []RecordedRequest
}

func NewFakeAPI(t testing.TB) *FakeAPI {
	api := &FakeAPI{handlers: map[string]http.HandlerFunc{}}
	api.server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.server.Close)
	return api
}

func (api *FakeAPI) URL() string {
	return api.server.URL
}

func (api *FakeAPI) Handle(method, path string, handler http.HandlerFunc) {
	api.mutex.Lock()
	defer api.mutex.Unlock()
	api.handlers[method+" "+path] = handler
}

func (api *FakeAPI) RespondWithFixture(t testing.TB, method, path string, status int, fixture string) {
	body := Fixture(t, fixture)
	api.Handle(method, path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	})
}

func (api *FakeAPI) RespondEmpty(method, path string, status int) {
	api.Handle(method, path, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
}

func (api *FakeAPI) Requests() []RecordedRequest {
	api.mutex.Lock()
	defer api.mutex.Unlock()
	return append([]RecordedRequest(nil), api.requests...)
}

func (api *FakeAPI) LastRequest(t testing.TB) RecordedRequest {
	t.Helper()
	requests := api.Requests()
	if len(requests) == 0 {
		t.Fatal("the fake API received no request")
	}
	return requests[len(requests)-1]
}

func (api *FakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	api.mutex.Lock()
	api.requests = append(api.requests, RecordedRequest{
		Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.Query(),
		Authorization: r.Header.Get("Authorization"), ContentType: r.Header.Get("Content-Type"), Body: body,
	})
	handler, ok := api.handlers[r.Method+" "+r.URL.EscapedPath()]
	api.mutex.Unlock()
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found"}}`))
		return
	}
	handler(w, r)
}
