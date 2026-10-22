// Package envconfig reads a module's configuration from environment variables and reports every problem at once (rule 1).
package envconfig

import (
	"errors"
	"net/url"
	"strings"
)

type Getenv func(string) string

// MinModuleSecretLength matches 128 bits of entropy encoded as base64url.
const MinModuleSecretLength = 22

type EnvReader struct {
	getenv  Getenv
	missing []string
	invalid []string
}

func NewEnvReader(getenv Getenv) *EnvReader {
	return &EnvReader{getenv: getenv}
}

func (reader *EnvReader) Required(name string) string {
	value := strings.TrimSpace(reader.getenv(name))
	if value == "" {
		reader.missing = append(reader.missing, name)
	}
	return value
}

func (reader *EnvReader) RequiredURL(name string) string {
	value := strings.TrimRight(reader.Required(name), "/")
	if value != "" && !isAbsoluteHTTPURL(value) {
		reader.MarkInvalid(name)
	}
	return value
}

// RequiredModuleSecret never echoes the value in its error, so a weak secret does not end up in logs.
func (reader *EnvReader) RequiredModuleSecret(name string) string {
	value := reader.Required(name)
	if value != "" && len(value) < MinModuleSecretLength {
		reader.MarkInvalid(name + " (at least 22 characters)")
	}
	return value
}

// MarkInvalid records a problem the caller detected; the description names the variable, never its value.
func (reader *EnvReader) MarkInvalid(description string) {
	reader.invalid = append(reader.invalid, description)
}

func (reader *EnvReader) Err() error {
	var problems []string
	if len(reader.missing) > 0 {
		problems = append(problems, "missing required environment variables: "+strings.Join(reader.missing, ", "))
	}
	if len(reader.invalid) > 0 {
		problems = append(problems, "invalid environment variables: "+strings.Join(reader.invalid, ", "))
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
