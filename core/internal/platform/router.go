package platform

import (
	"fmt"
	"net/http"
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
	kind       policyKind
	permission string
}

var (
	Public        = Policy{kind: policyPublic}
	Authenticated = Policy{kind: policyAuthenticated}
	// ModuleCredential guards routes that only module containers call, such as registration.
	ModuleCredential = Policy{kind: policyModuleCredential}
)

func RequirePermission(permission string) Policy {
	return Policy{kind: policyPermission, permission: permission}
}

type Authenticator interface {
	RequireAuthentication(next http.Handler) http.Handler
}

type Authorizer interface {
	RequirePermission(permission string, next http.Handler) http.Handler
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
		if policy.permission == "" {
			panic(fmt.Sprintf("route %q requires a permission name", pattern))
		}
		router.mux.Handle(pattern, router.authenticator.RequireAuthentication(router.authorizer.RequirePermission(policy.permission, handler)))
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
