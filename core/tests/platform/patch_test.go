package platform_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type patchBody struct {
	Status  platform.PatchField[string]   `json:"status"`
	Modules platform.PatchField[[]string] `json:"modules"`
}

func decodePatchBody(t *testing.T, raw string) patchBody {
	t.Helper()
	var body patchBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return body
}

func TestPatchFieldTellsOmittedNullAndValueApart(t *testing.T) {
	body := decodePatchBody(t, `{"status": null, "modules": ["alpha"]}`)

	if !body.Status.Present || !body.Status.Null || body.Status.HasValue() {
		t.Errorf("null field = %+v", body.Status)
	}
	if !body.Modules.HasValue() || !slices.Equal(body.Modules.Value, []string{"alpha"}) {
		t.Errorf("present field = %+v", body.Modules)
	}

	omitted := decodePatchBody(t, `{}`)
	if omitted.Status.Present || omitted.Modules.Present {
		t.Errorf("omitted fields = %+v", omitted)
	}
}

func TestPatchFieldRejectsMistypedValues(t *testing.T) {
	var body patchBody
	if err := json.Unmarshal([]byte(`{"status": 3}`), &body); err == nil {
		t.Error("expected an error for a number in a string field")
	}
}
