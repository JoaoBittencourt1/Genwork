package platform

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

type Getenv func(string) string

const Schema = "gentasks"

// MinModuleSecretLength matches 128 bits of entropy encoded as base64url.
const MinModuleSecretLength = 22

// MinEncryptionKeyBytes is the 256 bits the installer generates for GENTASKS_ENCRYPTION_KEY.
const MinEncryptionKeyBytes = 32

type ServeConfig struct {
	DatabaseURL     string
	HTTPAddr        string
	CoreInternalURL string
	ModuleSecret    string
	PublicURL       string
	EncryptionKey   []byte
	GithubWebURL    string
	GithubAPIURL    string
}

type envReader struct {
	getenv  Getenv
	missing []string
	invalid []string
}

func (r *envReader) required(name string) string {
	value := strings.TrimSpace(r.getenv(name))
	if value == "" {
		r.missing = append(r.missing, name)
	}
	return value
}

func (r *envReader) requiredURL(name string) string {
	value := strings.TrimRight(r.required(name), "/")
	if value != "" && !isAbsoluteHTTPURL(value) {
		r.invalid = append(r.invalid, name)
	}
	return value
}

// encryptionKey never echoes the value in its error, so a weak key does not end up in logs.
func (r *envReader) encryptionKey(name string) []byte {
	value := r.required(name)
	if value == "" {
		return nil
	}
	key, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(key) < MinEncryptionKeyBytes {
		r.invalid = append(r.invalid, name+" (at least 32 bytes as unpadded base64url)")
		return nil
	}
	return key
}

func (r *envReader) err() error {
	var problems []string
	if len(r.missing) > 0 {
		problems = append(problems, "missing required environment variables: "+strings.Join(r.missing, ", "))
	}
	if len(r.invalid) > 0 {
		problems = append(problems, "invalid environment variables: "+strings.Join(r.invalid, ", "))
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func LoadDatabaseURL(getenv Getenv) (string, error) {
	reader := &envReader{getenv: getenv}
	databaseURL := reader.required("GENTASKS_DATABASE_URL")
	return databaseURL, reader.err()
}

func LoadServeConfig(getenv Getenv) (ServeConfig, error) {
	reader := &envReader{getenv: getenv}
	config := ServeConfig{
		DatabaseURL:     reader.required("GENTASKS_DATABASE_URL"),
		HTTPAddr:        reader.required("GENTASKS_HTTP_ADDR"),
		CoreInternalURL: reader.requiredURL("CORE_INTERNAL_URL"),
		ModuleSecret:    reader.required("GENWORK_MODULE_SECRET"),
		PublicURL:       reader.requiredURL("GENWORK_PUBLIC_URL"),
		EncryptionKey:   reader.encryptionKey("GENTASKS_ENCRYPTION_KEY"),
		GithubWebURL:    reader.requiredURL("GITHUB_WEB_URL"),
		GithubAPIURL:    reader.requiredURL("GITHUB_API_URL"),
	}
	if config.ModuleSecret != "" && len(config.ModuleSecret) < MinModuleSecretLength {
		reader.invalid = append(reader.invalid, "GENWORK_MODULE_SECRET (at least 22 characters)")
	}
	return config, reader.err()
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
