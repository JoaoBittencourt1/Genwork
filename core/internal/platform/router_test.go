package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordingGuard struct{ authenticated, authorized []string }

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
	router := NewRouter(guard, guard)
	router.HandleFunc("GET /public", Public, okHandler)
	router.HandleFunc("GET /private", Authenticated, okHandler)
	router.HandleFunc("GET /admin", RequirePermission("core.invite.read"), okHandler)

	for _, path := range []string{"/public", "/private", "/admin"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	if len(guard.authenticated) != 2 || guard.authenticated[0] != "/private" || guard.authenticated[1] != "/admin" {
		t.Errorf("authenticated paths = %v", guard.authenticated)
	}
	if len(guard.authorized) != 1 || guard.authorized[0] != "core.invite.read" {
		t.Errorf("authorized permissions = %v", guard.authorized)
	}
}

func TestRouterPanicsWithoutPolicy(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a route without policy")
		}
	}()
	NewRouter(&recordingGuard{}, &recordingGuard{}).HandleFunc("GET /oops", Policy{}, okHandler)
}

func TestRouterPanicsOnEmptyPermission(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an empty permission")
		}
	}()
	NewRouter(&recordingGuard{}, &recordingGuard{}).HandleFunc("GET /oops", RequirePermission(""), okHandler)
}
