package platform

import (
	"bytes"
	"encoding/json"
)

// PatchField tells an omitted field apart from an explicit null in PATCH bodies (rule 17).
// It mirrors the SDK's type because core does not depend on the module SDK.
type PatchField[T any] struct {
	Present bool
	Null    bool
	Value   T
}

func (field *PatchField[T]) UnmarshalJSON(data []byte) error {
	field.Present = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Null = true
		return nil
	}
	return json.Unmarshal(data, &field.Value)
}

// HasValue reports a present, non-null field.
func (field PatchField[T]) HasValue() bool {
	return field.Present && !field.Null
}
