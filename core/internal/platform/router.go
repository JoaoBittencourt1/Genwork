package platform

import (
	"fmt"
	"net/http"
	"slices"
)

type policyKind int

const (
	policyUndeclared policyKind = iota
	policyPublic
	policyAuthenticated
	policyPermission
	policyModuleCredential
)

type Policy struct {
	kind        policyKind
	permissions []string
}

var (
	Public        = Policy{kind: policyPublic}
	Authenticated = Policy{kind: policyAuthenticated}
	// ModuleCredential guards routes that only module containers call, such as registration.
	ModuleCredential = Policy{kind: policyModuleCredential}
)

func RequirePermission(permission string) Policy {
	return RequireAnyPermission(permission)
}

// RequireAnyPermission lets the caller through when their role grants at least one of the permissions.
func RequireAnyPermission(permissions ...string) Policy {
	return Policy{kind: policyPermission, permissions: permissions}
}

type Authenticator interface {
	RequireAuthentication(next http.Handler) http.Handler
}

type Authorizer interface {
	RequireAnyPermission(permissions []string, next http.Handler) http.Handler
}

type ModuleAuthenticator interface {
	RequireModuleCredential(next http.Handler) http.Handler
}

type Router struct {
	mux                 *http.ServeMux
	authenticator       Authenticator
	authorizer          Authorizer
	moduleAuthenticator ModuleAuthenticator
}

func NewRouter(authenticator Authenticator, authorizer Authorizer, moduleAuthenticator ModuleAuthenticator) *Router {
	return &Router{
		mux: http.NewServeMux(), authenticator: authenticator, authorizer: authorizer, moduleAuthenticator: moduleAuthenticator,
	}
}

// Handle panics on an undeclared policy so that a route can never become public by omission.
func (router *Router) Handle(pattern string, policy Policy, handler http.Handler) {
	switch policy.kind {
	case policyPublic:
		router.mux.Handle(pattern, handler)
	case policyAuthenticated:
		router.mux.Handle(pattern, router.authenticator.RequireAuthentication(handler))
	case policyPermission:
		if len(policy.permissions) == 0 || slices.Contains(policy.permissions, "") {
			panic(fmt.Sprintf("route %q requires a permission name", pattern))
		}
		router.mux.Handle(pattern, router.authenticator.RequireAuthentication(router.authorizer.RequireAnyPermission(policy.permissions, handler)))
	case policyModuleCredential:
		router.mux.Handle(pattern, router.moduleAuthenticator.RequireModuleCredential(handler))
	default:
		panic(fmt.Sprintf("route %q has no access policy", pattern))
	}
}

func (router *Router) HandleFunc(pattern string, policy Policy, handler http.HandlerFunc) {
	router.Handle(pattern, policy, handler)
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}
