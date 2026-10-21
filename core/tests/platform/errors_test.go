package platform_test

import (
	"errors"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteErrorShapesAPIErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	platform.WriteError(recorder, httptest.NewRequest("GET", "/", nil), platform.NewValidationError(map[string]string{"email": "invalid"}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	expected := `{"error":{"code":"validation_failed","fields":{"email":"invalid"}}}` + "\n"
	if recorder.Body.String() != expected {
		t.Errorf("body = %q, want %q", recorder.Body.String(), expected)
	}
}

func TestWriteErrorHidesUnexpectedErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	platform.WriteError(recorder, httptest.NewRequest("GET", "/", nil), errors.New("connection refused to 10.0.0.5"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "10.0.0.5") {
		t.Errorf("internal details leaked: %s", recorder.Body.String())
	}
}

func TestDecodeJSONRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	var target struct {
		Email string `json:"email"`
	}
	for _, body := range []string{`{"email":"a","extra":1}`, `{"email":"a"}{}`, `not json`} {
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		if err := platform.DecodeJSON(httptest.NewRecorder(), request, &target); err != platform.ErrMalformedBody {
			t.Errorf("DecodeJSON(%q) = %v, want platform.ErrMalformedBody", body, err)
		}
	}
}

func TestAPIErrorMessageIsItsCode(t *testing.T) {
	if got := platform.NewAPIError(http.StatusConflict, "invite_not_pending").Error(); got != "invite_not_pending" {
		t.Errorf("Error() = %q", got)
	}
}
