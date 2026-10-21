package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

type unavailableVerifier struct{}

func (unavailableVerifier) Verify(context.Context, string) (tokens.Principal, error) {
	return tokens.Principal{}, tokens.ErrKeysUnavailable
}

func newRouter(t *testing.T) (*httpapi.Router, *coretest.FakeCore) {
	t.Helper()
	core := coretest.NewFakeCore(t)
	verifier := tokens.NewTokenVerifier(tokens.TokenVerifierConfig{JWKSURL: core.JWKSURL(), Issuer: coretest.Issuer})
	router := httpapi.NewRouter(verifier)
	router.HandleFunc("GET /public", httpapi.Public, writePrincipal)
	router.HandleFunc("GET /signed-in", httpapi.Authenticated, writePrincipal)
	router.HandleFunc("GET /boards", httpapi.RequirePermission("gentasks.board.read"), writePrincipal)
	return router, core
}

func writePrincipal(w http.ResponseWriter, r *http.Request) {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"user": principal.UserToken})
}

func serve(router http.Handler, path, accessToken string) *httptest.ResponseRecorder {
	request := httptest.NewRequest("GET", path, nil)
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return body.Error.Code
}

func TestRouterAppliesPolicies(t *testing.T) {
	router, core := newRouter(t)
	reader := core.MintAccessToken(t, "reader", "gentasks.board.read")
	outsider := core.MintAccessToken(t, "outsider")

	if recorder := serve(router, "/public", ""); recorder.Code != http.StatusOK {
		t.Errorf("public status = %d", recorder.Code)
	}
	if recorder := serve(router, "/signed-in", ""); recorder.Code != http.StatusUnauthorized || errorCode(t, recorder) != "invalid_token" {
		t.Errorf("anonymous status = %d %s", recorder.Code, recorder.Body)
	}
	if recorder := serve(router, "/signed-in", "garbage"); recorder.Code != http.StatusUnauthorized {
		t.Errorf("garbage token status = %d", recorder.Code)
	}
	if recorder := serve(router, "/signed-in", outsider); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "outsider") {
		t.Errorf("authenticated = %d %s", recorder.Code, recorder.Body)
	}
	if recorder := serve(router, "/boards", outsider); recorder.Code != http.StatusForbidden || errorCode(t, recorder) != "forbidden" {
		t.Errorf("missing permission = %d %s", recorder.Code, recorder.Body)
	}
	if recorder := serve(router, "/boards", reader); recorder.Code != http.StatusOK {
		t.Errorf("granted permission = %d %s", recorder.Code, recorder.Body)
	}
}

func TestRouterAnswersServiceUnavailableWhenKeysCannotBeFetched(t *testing.T) {
	router := httpapi.NewRouter(unavailableVerifier{})
	router.HandleFunc("GET /signed-in", httpapi.Authenticated, writePrincipal)

	recorder := serve(router, "/signed-in", "any")

	if recorder.Code != http.StatusServiceUnavailable || errorCode(t, recorder) != "service_unavailable" {
		t.Errorf("status = %d %s", recorder.Code, recorder.Body)
	}
}

func TestRouterPanicsWithoutPolicy(t *testing.T) {
	for name, policy := range map[string]httpapi.Policy{"undeclared": {}, "empty permission": httpapi.RequirePermission("")} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			httpapi.NewRouter(unavailableVerifier{}).HandleFunc("GET /oops", policy, writePrincipal)
		})
	}
}
