package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/mail/mailtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testPublicURL    = "http://localhost:8485"
	testInstanceName = "Genwork Test"
	testPassword     = "correct horse battery"
)

var testPasswordParams = users.PasswordParams{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

var generousRateLimits = RateLimits{
	Login:       platform.RateLimit{PerMinute: 10000, Burst: 10000},
	Refresh:     platform.RateLimit{PerMinute: 10000, Burst: 10000},
	Invitations: platform.RateLimit{PerMinute: 10000, Burst: 10000},
}

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
	clock   *testClock
	mailer  *mailtest.Recorder
}

type testAppOption func(*Settings)

func withRateLimits(limits RateLimits) testAppOption {
	return func(settings *Settings) { settings.RateLimits = limits }
}

func withPasswordParams(params users.PasswordParams) testAppOption {
	return func(settings *Settings) { settings.PasswordParams = params }
}

func newTestApp(t *testing.T, options ...testAppOption) *testApp {
	t.Helper()
	signingKey, err := auth.ParseSigningKey(auth.GenerateSigningKeySeed())
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	settings := Settings{
		Version:        "test",
		PublicURL:      testPublicURL,
		InstanceName:   testInstanceName,
		DefaultLocale:  "pt-BR",
		SigningKey:     signingKey,
		TrustedProxies: platform.NewClientAddressResolver(nil),
		PasswordParams: testPasswordParams,
		RateLimits:     generousRateLimits,
	}
	for _, option := range options {
		option(&settings)
	}
	clock := &testClock{current: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	mailer := &mailtest.Recorder{}
	db := dbtest.NewMigratedSchema(t)
	return &testApp{
		t:       t,
		handler: New(settings, Dependencies{DB: db, Mailer: mailer, Now: clock.Now}),
		db:      db,
		clock:   clock,
		mailer:  mailer,
	}
}

type request struct {
	method      string
	path        string
	body        any
	accessToken string
	webClient   bool
	cookies     []*http.Cookie
}

type response struct {
	status  int
	body    []byte
	header  http.Header
	cookies []*http.Cookie
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
	if req.accessToken != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+req.accessToken)
	}
	if req.webClient {
		httpRequest.Header.Set(auth.ClientHeader, "web")
	}
	for _, cookie := range req.cookies {
		httpRequest.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	app.handler.ServeHTTP(recorder, httpRequest)
	result := recorder.Result()
	return response{status: recorder.Code, body: recorder.Body.Bytes(), header: recorder.Header(), cookies: result.Cookies()}
}

func (resp response) decode(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.body, target); err != nil {
		t.Fatalf("decode %q: %v", resp.body, err)
	}
}

func (resp response) errorCode(t *testing.T) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	resp.decode(t, &body)
	return body.Error.Code
}

func (resp response) cookie(name string) *http.Cookie {
	for _, cookie := range resp.cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func expectStatus(t *testing.T, resp response, status int) {
	t.Helper()
	if resp.status != status {
		t.Fatalf("status = %d, want %d; body = %s", resp.status, status, resp.body)
	}
}

func expectError(t *testing.T, resp response, status int, code string) {
	t.Helper()
	expectStatus(t, resp, status)
	if got := resp.errorCode(t); got != code {
		t.Fatalf("error code = %q, want %q", got, code)
	}
}

func (app *testApp) createUser(email, name, roleName string) users.User {
	return app.createUserWithParams(email, name, roleName, testPasswordParams)
}

func (app *testApp) createUserWithParams(email, name, roleName string, params users.PasswordParams) users.User {
	app.t.Helper()
	ctx := context.Background()
	role, err := access.RoleRepository{}.FindByName(ctx, app.db, roleName)
	if err != nil {
		app.t.Fatalf("find role %s: %v", roleName, err)
	}
	user, err := users.NewService(users.NewPasswordHasher(params, 1)).Create(ctx, app.db, users.CreateInput{
		Email: email, Name: name, Password: testPassword, RoleID: role.ID,
	})
	if err != nil {
		app.t.Fatalf("create user: %v", err)
	}
	return user
}

func (app *testApp) createRole(name string, permissions ...string) {
	app.t.Helper()
	ctx := context.Background()
	if _, err := app.db.Exec(ctx, `INSERT INTO roles (name) VALUES ($1)`, name); err != nil {
		app.t.Fatalf("create role: %v", err)
	}
	for _, permission := range permissions {
		_, err := app.db.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT roles.id, permissions.id FROM roles, permissions WHERE roles.name = $1 AND permissions.name = $2`,
			name, permission)
		if err != nil {
			app.t.Fatalf("grant permission: %v", err)
		}
	}
}

type signInBody struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken"`
	Session              string    `json:"session"`
}

func (app *testApp) loginNative(email string) signInBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/core/v1/auth/login", body: map[string]any{
		"email": email, "password": testPassword, "client": "native",
	}})
	expectStatus(app.t, resp, http.StatusOK)
	var body signInBody
	resp.decode(app.t, &body)
	return body
}

func (app *testApp) roleToken(name string) string {
	app.t.Helper()
	role, err := access.RoleRepository{}.FindByName(context.Background(), app.db, name)
	if err != nil {
		app.t.Fatalf("find role %s: %v", name, err)
	}
	return role.Token
}
