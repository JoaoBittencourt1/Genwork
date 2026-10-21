package platform_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
)

func TestSendWithTokenSendsTheBearerTokenQueryAndJSONBody(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.RespondWithFixture(t, http.MethodPost, "/api/things", http.StatusCreated, "boards.json")
	client := platform.NewAPIClient(api.URL()+"/", http.DefaultClient)

	body, err := client.SendWithToken(context.Background(), platform.APIRequest{
		Method: http.MethodPost, Path: "/api/things", Query: url.Values{"archived": {"true"}}, Body: map[string]string{"name": "x"},
	}, "access-1")

	if err != nil {
		t.Fatalf("SendWithToken: %v", err)
	}
	if string(body) != string(apitest.Fixture(t, "boards.json")) {
		t.Fatalf("body = %s", body)
	}
	request := api.LastRequest(t)
	if request.Authorization != "Bearer access-1" || request.ContentType != "application/json" {
		t.Fatalf("headers = %q, %q", request.Authorization, request.ContentType)
	}
	if request.Query.Get("archived") != "true" || string(request.Body) != `{"name":"x"}` {
		t.Fatalf("query = %v, body = %s", request.Query, request.Body)
	}
}

func TestSendWithTokenAnswersNilForAnEmptyBodyAndOmitsAuthorizationWithoutAToken(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.RespondEmpty(http.MethodDelete, "/api/things/1", http.StatusNoContent)
	client := platform.NewAPIClient(api.URL(), http.DefaultClient)

	body, err := client.SendWithToken(context.Background(), platform.APIRequest{Method: http.MethodDelete, Path: "/api/things/1"}, "")

	if err != nil || body != nil {
		t.Fatalf("body = %s, err = %v", body, err)
	}
	if request := api.LastRequest(t); request.Authorization != "" || request.ContentType != "" {
		t.Fatalf("headers = %q, %q", request.Authorization, request.ContentType)
	}
}

func TestSendWithTokenMapsTheErrorBodyToAnAPIError(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.RespondWithFixture(t, http.MethodPost, "/api/things", http.StatusBadRequest, "validation_error.json")
	client := platform.NewAPIClient(api.URL(), http.DefaultClient)

	_, err := client.SendWithToken(context.Background(), platform.APIRequest{Method: http.MethodPost, Path: "/api/things"}, "t")

	var apiError *platform.APIError
	if !errors.As(err, &apiError) || apiError.Code != "validation_failed" || apiError.Fields["title"] != "invalid" {
		t.Fatalf("err = %#v", err)
	}
	if err.Error() != "genwork API answered 400 validation_failed (title: invalid, urgency: invalid)" {
		t.Fatalf("message = %q", err.Error())
	}
	if !platform.IsAPIStatus(err, http.StatusBadRequest) || platform.IsAPIStatus(err, http.StatusNotFound) {
		t.Fatal("IsAPIStatus does not match the status")
	}
}

func TestSendWithTokenNamesTheStatusWhenTheErrorBodyIsNotTheAPIShape(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.RespondEmpty(http.MethodGet, "/api/down", http.StatusBadGateway)
	client := platform.NewAPIClient(api.URL(), http.DefaultClient)

	_, err := client.SendWithToken(context.Background(), platform.APIRequest{Method: http.MethodGet, Path: "/api/down"}, "t")

	if err == nil || err.Error() != "genwork API answered 502 bad_gateway" {
		t.Fatalf("err = %v", err)
	}
}

func TestSendWithTokenReportsAnUnreachableInstance(t *testing.T) {
	client := platform.NewAPIClient("http://127.0.0.1:1", http.DefaultClient)

	_, err := client.SendWithToken(context.Background(), platform.APIRequest{Method: http.MethodGet, Path: "/api"}, "t")

	if !errors.Is(err, platform.ErrInstanceUnavailable) {
		t.Fatalf("err = %v, want ErrInstanceUnavailable", err)
	}
}

func TestSendWithTokenRejectsABodyThatCannotBeEncoded(t *testing.T) {
	client := platform.NewAPIClient("http://127.0.0.1:1", http.DefaultClient)

	_, err := client.SendWithToken(context.Background(), platform.APIRequest{Method: http.MethodPost, Path: "/api", Body: make(chan int)}, "t")

	if err == nil || errors.Is(err, platform.ErrInstanceUnavailable) {
		t.Fatalf("err = %v, want an encoding error", err)
	}
}

func TestExpandPathEscapesEveryToken(t *testing.T) {
	path := platform.ExpandPath("/api/boards/%s/tasks/%s", "a/b", "c?d")
	if path != "/api/boards/a%2Fb/tasks/c%3Fd" {
		t.Fatalf("path = %q", path)
	}
}
