package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

func TestWriteErrorUsesCoreErrorBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpapi.WriteError(recorder, httptest.NewRequest("GET", "/", nil), httpapi.NewValidationError(map[string]string{"name": "required"}))

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d", recorder.Code)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"error":{"code":"validation_failed","fields":{"name":"required"}}}` {
		t.Errorf("body = %s", body)
	}
}

func TestWriteErrorHidesUnexpectedErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpapi.WriteError(recorder, httptest.NewRequest("GET", "/", nil), errors.New("database password is hunter2"))

	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "hunter2") {
		t.Errorf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	if httpapi.ErrNotFound.Error() != "not_found" {
		t.Errorf("error text = %s", httpapi.ErrNotFound.Error())
	}
}

func TestDecodeJSONRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	type nameRequest struct {
		Name string `json:"name"`
	}
	for _, body := range []string{`{"name":"a","admin":true}`, `{"name":"a"} {}`, `not json`} {
		var target nameRequest
		recorder := httptest.NewRecorder()
		err := httpapi.DecodeJSON(recorder, httptest.NewRequest("POST", "/", strings.NewReader(body)), &target)
		if err != httpapi.ErrMalformedBody {
			t.Errorf("%s: error = %v", body, err)
		}
	}
	var target nameRequest
	if err := httpapi.DecodeJSON(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok"}`)), &target); err != nil || target.Name != "ok" {
		t.Errorf("valid body: %v %+v", err, target)
	}
}
