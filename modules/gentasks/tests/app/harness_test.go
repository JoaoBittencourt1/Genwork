package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userToken = "ana-user-token"

type testClock struct {
	mutex   sync.Mutex
	current time.Time
}

func (clock *testClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.current
}

func (clock *testClock) Advance(duration time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.current = clock.current.Add(duration)
}

type testApp struct {
	t       *testing.T
	handler http.Handler
	db      *pgxpool.Pool
	core    *coretest.FakeCore
	github  *githubsynctest.FakeGithub
	clock   *testClock
	token   string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	github := githubsynctest.NewFakeGithub(t)
	testApp := newTestAppWithGithubAPI(t, github.URL())
	testApp.github = github
	return testApp
}

func newTestAppWithGithubAPI(t *testing.T, githubAPIURL string) *testApp {
	t.Helper()
	db := dbtest.NewMigratedSchema(t)
	core := coretest.NewFakeCore(t)
	core.AddUser(coreclient.DirectoryUser{Token: userToken, Name: "Ana", Email: "ana@example.com"})
	clock := &testClock{current: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	handler, err := app.New(app.Dependencies{
		DB:            db,
		TokenVerifier: tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: coretest.Issuer}),
		UserDirectory: coreclient.NewUserDirectoryClient(core.URL(), time.Second),
		Github: app.GithubConfig{
			EncryptionKey: githubsynctest.EncryptionKey,
			PublicURL:     coretest.Issuer,
			WebURL:        githubsynctest.UnroutableGithubWebURL,
			APIURL:        githubAPIURL,
		},
		Now: clock.Now,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	testApp := &testApp{t: t, handler: handler, db: db, core: core, clock: clock}
	testApp.token = testApp.tokenWith(access.Permissions...)
	return testApp
}

func (app *testApp) tokenWith(permissions ...string) string {
	return app.core.MintAccessToken(app.t, userToken, permissions...)
}

type request struct {
	method      string
	path        string
	body        any
	rawBody     string
	rawBytes    []byte
	accessToken string
	anonymous   bool
	headers     map[string]string
}

type response struct {
	status int
	body   []byte
}

func (app *testApp) do(req request) response {
	app.t.Helper()
	var reader io.Reader
	switch {
	case req.rawBytes != nil:
		reader = bytes.NewReader(req.rawBytes)
	case req.rawBody != "":
		reader = bytes.NewReader([]byte(req.rawBody))
	case req.body != nil:
		encoded, err := json.Marshal(req.body)
		if err != nil {
			app.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	httpRequest := httptest.NewRequest(req.method, req.path, reader)
	httpRequest.Header.Set("Content-Type", "application/json")
	for name, value := range req.headers {
		httpRequest.Header.Set(name, value)
	}
	if !req.anonymous {
		accessToken := req.accessToken
		if accessToken == "" {
			accessToken = app.token
		}
		httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	}
	recorder := httptest.NewRecorder()
	app.handler.ServeHTTP(recorder, httpRequest)
	return response{status: recorder.Code, body: recorder.Body.Bytes()}
}

func (resp response) decode(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.body, target); err != nil {
		t.Fatalf("decode %q: %v", resp.body, err)
	}
}

type errorBody struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

func expectStatus(t *testing.T, resp response, status int) {
	t.Helper()
	if resp.status != status {
		t.Fatalf("status = %d, want %d; body = %s", resp.status, status, resp.body)
	}
}

func expectError(t *testing.T, resp response, status int, code string) errorBody {
	t.Helper()
	expectStatus(t, resp, status)
	var body errorBody
	resp.decode(t, &body)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q; body = %s", body.Error.Code, code, resp.body)
	}
	return body
}

func expectValidationError(t *testing.T, resp response, field string) {
	t.Helper()
	body := expectError(t, resp, http.StatusBadRequest, "validation_failed")
	if _, found := body.Error.Fields[field]; !found {
		t.Fatalf("validation fields = %v, want %s", body.Error.Fields, field)
	}
}
