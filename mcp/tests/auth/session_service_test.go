package auth_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth/authtest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
)

type sessionFixture struct {
	api   *apitest.FakeAPI
	core  *authtest.FakeSessionCore
	store *authtest.MemoryCredentialStore
	lock  string
}

func newSessionFixture(t *testing.T) sessionFixture {
	api := apitest.NewFakeAPI(t)
	return sessionFixture{
		api: api, core: authtest.NewFakeSessionCore(api), store: authtest.NewMemoryCredentialStore(),
		lock: filepath.Join(t.TempDir(), "refresh.lock"),
	}
}

// newService builds one genwork-mcp process; several share the store and lock like processes share the keyring.
func (fixture sessionFixture) newService(now func() time.Time) *auth.SessionService {
	return auth.NewSessionService(auth.SessionServiceConfig{
		InstanceURL: fixture.api.URL(),
		Client:      auth.NewSessionClient(platform.NewAPIClient(fixture.api.URL(), http.DefaultClient), "genwork-mcp on test"),
		Store:       fixture.store,
		Lock:        auth.NewFileRefreshLock(fixture.lock),
		Now:         now,
	})
}

func TestLoginOpensANativeSessionAndStoresOnlyTheRefreshToken(t *testing.T) {
	fixture := newSessionFixture(t)
	service := fixture.newService(nil)

	if err := service.Login(context.Background(), " "+authtest.Email+" ", authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if fixture.core.LastLogin["client"] != "native" || fixture.core.LastLogin["deviceName"] != "genwork-mcp on test" {
		t.Fatalf("login request = %v", fixture.core.LastLogin)
	}
	credential, err := fixture.store.Load(fixture.api.URL())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := auth.StoredCredential{Email: authtest.Email, Session: authtest.Session, RefreshToken: "refresh-1"}
	if credential != want {
		t.Fatalf("credential = %+v, want %+v", credential, want)
	}
	accessToken, err := service.AccessToken(context.Background())
	if err != nil || accessToken != "access-1" || fixture.core.Refreshes != 0 {
		t.Fatalf("access token = %q, err = %v, refreshes = %d", accessToken, err, fixture.core.Refreshes)
	}
}

func TestLoginRejectsMissingCredentialsWithoutCallingCore(t *testing.T) {
	fixture := newSessionFixture(t)

	err := fixture.newService(nil).Login(context.Background(), " ", "")

	if !errors.Is(err, auth.ErrMissingCredentials) || len(fixture.api.Requests()) != 0 {
		t.Fatalf("err = %v, requests = %d", err, len(fixture.api.Requests()))
	}
}

func TestLoginSurfacesAWrongPassword(t *testing.T) {
	fixture := newSessionFixture(t)

	err := fixture.newService(nil).Login(context.Background(), authtest.Email, "wrong")

	if !platform.IsAPIStatus(err, http.StatusUnauthorized) {
		t.Fatalf("err = %v, want 401", err)
	}
	if _, err := fixture.store.Load(fixture.api.URL()); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("a credential was stored after a failed login: %v", err)
	}
}

func TestAccessTokenRefreshesShortlyBeforeExpiryAndRotatesTheStoredToken(t *testing.T) {
	fixture := newSessionFixture(t)
	now := time.Now()
	clock := func() time.Time { return now }
	fixture.core.Now = clock
	service := fixture.newService(clock)
	if err := service.Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}

	now = now.Add(15*time.Minute - auth.RefreshMargin)
	accessToken, err := service.AccessToken(context.Background())

	if err != nil || accessToken != "access-2" {
		t.Fatalf("access token = %q, err = %v", accessToken, err)
	}
	if credential, _ := fixture.store.Load(fixture.api.URL()); credential.RefreshToken != "refresh-2" {
		t.Fatalf("stored refresh token = %q, want the rotated one", credential.RefreshToken)
	}
}

func TestProcessesSharingTheStoreNeverReuseARefreshToken(t *testing.T) {
	fixture := newSessionFixture(t)
	if err := fixture.newService(nil).Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}
	processes := []*auth.SessionService{fixture.newService(nil), fixture.newService(nil), fixture.newService(nil)}

	var group sync.WaitGroup
	errs := make([]error, len(processes))
	for index, process := range processes {
		group.Go(func() { _, errs[index] = process.AccessToken(context.Background()) })
	}
	group.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("AccessToken: %v", err)
		}
	}
	if fixture.core.IsRevoked() || fixture.core.Refreshes != len(processes) {
		t.Fatalf("revoked = %v, refreshes = %d", fixture.core.IsRevoked(), fixture.core.Refreshes)
	}
}

func TestAccessTokenReportsARevokedSession(t *testing.T) {
	fixture := newSessionFixture(t)
	if err := fixture.newService(nil).Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}
	fixture.core.Revoke()

	_, err := fixture.newService(nil).AccessToken(context.Background())

	if !errors.Is(err, auth.ErrSessionEnded) {
		t.Fatalf("err = %v, want ErrSessionEnded", err)
	}
}

func TestAccessTokenWithoutLoginAsksToSignIn(t *testing.T) {
	fixture := newSessionFixture(t)

	_, err := fixture.newService(nil).AccessToken(context.Background())

	if !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("err = %v, want ErrNotSignedIn", err)
	}
}

func TestInvalidateForcesARefreshOnlyForTheRejectedToken(t *testing.T) {
	fixture := newSessionFixture(t)
	service := fixture.newService(nil)
	if err := service.Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}

	service.Invalidate("some-other-token")
	unchanged, _ := service.AccessToken(context.Background())
	service.Invalidate(unchanged)
	refreshed, err := service.AccessToken(context.Background())

	if unchanged != "access-1" || refreshed != "access-2" || err != nil {
		t.Fatalf("unchanged = %q, refreshed = %q, err = %v", unchanged, refreshed, err)
	}
}

func TestLogoutRevokesTheSessionAndForgetsTheCredential(t *testing.T) {
	fixture := newSessionFixture(t)
	service := fixture.newService(nil)
	if err := service.Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := service.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if !fixture.core.LoggedOut {
		t.Fatal("core was not asked to revoke the session")
	}
	if _, err := service.SignedInEmail(); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("SignedInEmail after logout: %v", err)
	}
}

func TestLogoutForgetsTheCredentialWhenCoreAlreadyEndedTheSession(t *testing.T) {
	fixture := newSessionFixture(t)
	if err := fixture.newService(nil).Login(context.Background(), authtest.Email, authtest.Password); err != nil {
		t.Fatalf("Login: %v", err)
	}
	fixture.core.Revoke()

	if err := fixture.newService(nil).Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if _, err := fixture.store.Load(fixture.api.URL()); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("credential kept after logout: %v", err)
	}
}

func TestLogoutWithoutLoginAsksToSignIn(t *testing.T) {
	fixture := newSessionFixture(t)

	if err := fixture.newService(nil).Logout(context.Background()); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("err = %v, want ErrNotSignedIn", err)
	}
}
