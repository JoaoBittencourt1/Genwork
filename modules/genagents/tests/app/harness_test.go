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

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelinktest"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
	"github.com/jackc/pgx/v5/pgxpool"
)

const adminUserToken = "ana-admin-token"

var testEncryptionKey = bytes.Repeat([]byte{9}, 32)

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
	coreAPI *corelinktest.FakeCore
	clock   *testClock
	token   string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	db := dbtest.NewMigratedSchema(t)
	core := coretest.NewFakeCore(t)
	clock := &testClock{current: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
	coreAPI := corelinktest.NewFakeCore(t, clock.Now)
	handler, err := app.New(app.Dependencies{
		DB:            db,
		TokenVerifier: tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: coretest.Issuer}),
		EncryptionKey: testEncryptionKey,
		Core:          corelink.NewClient(coreAPI.URL(), time.Second),
		GenworkMCPURL: "http://genwork.test/mcp",
		ClaimLongPoll: 150 * time.Millisecond,
		Now:           clock.Now,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	testApp := &testApp{t: t, handler: handler, db: db, core: core, coreAPI: coreAPI, clock: clock}
	testApp.token = testApp.tokenWith(access.Permissions...)
	return testApp
}

func (app *testApp) tokenWith(permissions ...string) string {
	return app.core.MintAccessToken(app.t, adminUserToken, permissions...)
}

type request struct {
	method      string
	path        string
	body        any
	accessToken string
	anonymous   bool
	bearer      string
}

type response struct {
	status  int
	body    []byte
	headers http.Header
}

func (resp response) header(name string) string {
	return resp.headers.Get(name)
}

func (app *testApp) do(req request) response {
	app.t.Helper()
	var reader io.Reader
	if req.body != nil {
		encoded, err := json.Marshal(req.body)
		if err != nil {
			app.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	httpRequest := httptest.NewRequest(req.method, req.path, reader)
	httpRequest.Header.Set("Content-Type", "application/json")
	switch {
	case req.bearer != "":
		httpRequest.Header.Set("Authorization", "Bearer "+req.bearer)
	case !req.anonymous:
		accessToken := req.accessToken
		if accessToken == "" {
			accessToken = app.token
		}
		httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	}
	recorder := httptest.NewRecorder()
	app.handler.ServeHTTP(recorder, httpRequest)
	return response{status: recorder.Code, body: recorder.Body.Bytes(), headers: recorder.Header()}
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
