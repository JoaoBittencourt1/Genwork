package httpapi_test

import (
	"encoding/json"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type patchTaskRequest struct {
	Title       httpapi.PatchField[string] `json:"title"`
	Description httpapi.PatchField[string] `json:"description"`
}

func TestPatchFieldDistinguishesOmittedNullAndValue(t *testing.T) {
	var request patchTaskRequest
	if err := json.Unmarshal([]byte(`{"description":null}`), &request); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if request.Title.Present || request.Title.HasValue() {
		t.Errorf("omitted title = %+v", request.Title)
	}
	if !request.Description.Present || !request.Description.Null || request.Description.HasValue() {
		t.Errorf("null description = %+v", request.Description)
	}
}

func TestPatchFieldReadsValue(t *testing.T) {
	var request patchTaskRequest
	if err := json.Unmarshal([]byte(`{"title":"Ship it"}`), &request); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !request.Title.HasValue() || request.Title.Value != "Ship it" {
		t.Errorf("title = %+v", request.Title)
	}
	if err := json.Unmarshal([]byte(`{"title":42}`), &request); err == nil {
		t.Error("a value of the wrong type must fail")
	}
}
