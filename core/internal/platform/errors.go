package platform

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

type APIError struct {
	Status int
	Code   string
	Fields map[string]string
}

func (e *APIError) Error() string {
	return e.Code
}

func NewAPIError(status int, code string) *APIError {
	return &APIError{Status: status, Code: code}
}

func NewValidationError(fields map[string]string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "validation_failed", Fields: fields}
}

var (
	ErrInvalidToken  = NewAPIError(http.StatusUnauthorized, "invalid_token")
	ErrForbidden     = NewAPIError(http.StatusForbidden, "forbidden")
	ErrNotFound      = NewAPIError(http.StatusNotFound, "not_found")
	ErrMalformedBody = NewAPIError(http.StatusBadRequest, "malformed_body")
	ErrRateLimited   = NewAPIError(http.StatusTooManyRequests, "rate_limited")
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		slog.ErrorContext(r.Context(), "unhandled error", "method", r.Method, "path", r.URL.Path, "err", err)
		apiErr = NewAPIError(http.StatusInternalServerError, "internal_error")
	}
	WriteJSON(w, apiErr.Status, errorBody{Error: errorPayload{Code: apiErr.Code, Fields: apiErr.Fields}})
}

const maxRequestBodyBytes = 1 << 20

func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrMalformedBody
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrMalformedBody
	}
	return nil
}
