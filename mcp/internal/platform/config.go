package platform

import (
	"errors"
	"net/url"
	"strings"
)

type Getenv func(string) string

const InstanceURLVariable = "GENWORK_URL"

var (
	ErrMissingInstanceURL = errors.New("missing required environment variable " + InstanceURLVariable + " (the address of your Genwork gateway)")
	ErrInvalidInstanceURL = errors.New(InstanceURLVariable + " must be an absolute http or https URL")
)

type Config struct {
	InstanceURL string
}

func LoadConfig(getenv Getenv) (Config, error) {
	instanceURL := strings.TrimRight(strings.TrimSpace(getenv(InstanceURLVariable)), "/")
	if instanceURL == "" {
		return Config{}, ErrMissingInstanceURL
	}
	if !isAbsoluteHTTPURL(instanceURL) {
		return Config{}, ErrInvalidInstanceURL
	}
	return Config{InstanceURL: instanceURL}, nil
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
