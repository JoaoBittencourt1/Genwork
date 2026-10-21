package platform_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordingGuard struct{ authenticated, authorized, moduleCalls []string }

func (guard *recordingGuard) RequireModuleCredential(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guard.moduleCalls = append(guard.moduleCalls, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (guard *recordingGuard) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guard.authenticated = append(guard.authenticated, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (guard *recordingGuard) RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guard.authorized = append(guard.authorized, permission)
		next.ServeHTTP(w, r)
	})
}

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRouterAppliesDeclaredPolicies(t *testing.T) {
	guard := &recordingGuard{}
	router := platform.NewRouter(guard, guard, guard)
	router.HandleFunc("GET /public", platform.Public, okHandler)
	router.HandleFunc("GET /private", platform.Authenticated, okHandler)
	router.HandleFunc("GET /admin", platform.RequirePermission("core.invite.read"), okHandler)
	router.HandleFunc("GET /internal", platform.ModuleCredential, okHandler)

	for _, path := range []string{"/public", "/private", "/admin", "/internal"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	if len(guard.authenticated) != 2 || guard.authenticated[0] != "/private" || guard.authenticated[1] != "/admin" {
		t.Errorf("authenticated paths = %v", guard.authenticated)
	}
	if len(guard.authorized) != 1 || guard.authorized[0] != "core.invite.read" {
		t.Errorf("authorized permissions = %v", guard.authorized)
	}
	if len(guard.moduleCalls) != 1 || guard.moduleCalls[0] != "/internal" {
		t.Errorf("module credential paths = %v", guard.moduleCalls)
	}
}

func TestRouterPanicsWithoutPolicy(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a route without policy")
		}
	}()
	platform.NewRouter(&recordingGuard{}, &recordingGuard{}, &recordingGuard{}).HandleFunc("GET /oops", platform.Policy{}, okHandler)
}

func TestRouterPanicsOnEmptyPermission(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an empty permission")
		}
	}()
	platform.NewRouter(&recordingGuard{}, &recordingGuard{}, &recordingGuard{}).HandleFunc("GET /oops", platform.RequirePermission(""), okHandler)
}
