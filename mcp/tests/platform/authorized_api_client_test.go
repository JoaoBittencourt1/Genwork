package platform_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
)

func TestAuthorizedSendRetriesOnceWithAFreshTokenAfterA401(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.Handle(http.MethodGet, "/api/core/v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write(apitest.Fixture(t, "me_admin.json"))
	})
	tokens := apitest.NewStaticTokenSource("stale", "fresh")
	client := platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), tokens)

	body, err := client.Send(context.Background(), platform.APIRequest{Method: http.MethodGet, Path: "/api/core/v1/me"})

	if err != nil || len(body) == 0 {
		t.Fatalf("body = %s, err = %v", body, err)
	}
	if !slices.Equal(tokens.Invalidated, []string{"stale"}) || len(api.Requests()) != 2 {
		t.Fatalf("invalidated = %v, requests = %d", tokens.Invalidated, len(api.Requests()))
	}
}

func TestAuthorizedSendDoesNotRetryOtherErrors(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	tokens := apitest.NewStaticTokenSource("token")
	client := platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), tokens)

	_, err := client.Send(context.Background(), platform.APIRequest{Method: http.MethodGet, Path: "/api/missing"})

	if !platform.IsAPIStatus(err, http.StatusNotFound) || len(api.Requests()) != 1 || len(tokens.Invalidated) != 0 {
		t.Fatalf("err = %v, requests = %d", err, len(api.Requests()))
	}
}

func TestAuthorizedSendStopsWhenNoTokenIsAvailable(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	tokens := apitest.NewStaticTokenSource("token")
	tokens.Err = errors.New("not signed in")
	client := platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), tokens)

	_, err := client.Send(context.Background(), platform.APIRequest{Method: http.MethodGet, Path: "/api"})

	if !errors.Is(err, tokens.Err) || len(api.Requests()) != 0 {
		t.Fatalf("err = %v, requests = %d", err, len(api.Requests()))
	}
}
