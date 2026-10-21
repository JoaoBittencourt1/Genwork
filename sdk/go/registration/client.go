// Package registration announces a module to core on startup (rule 58).
package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

const (
	DefaultInitialBackoff = time.Second
	DefaultMaxBackoff     = 30 * time.Second
)

var ErrRegistrationRejected = errors.New("core rejected the module registration")

// RejectedError is permanent: retrying the same manifest cannot succeed.
type RejectedError struct {
	Status int
	Code   string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("%s: status %d, code %q", ErrRegistrationRejected, e.Status, e.Code)
}

func (e *RejectedError) Unwrap() error {
	return ErrRegistrationRejected
}

type RegisteredModule struct {
	Token   string `json:"token"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
}

type RegistrationClientConfig struct {
	CoreURL        string
	ModuleSecret   string
	HTTPClient     *http.Client
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type RegistrationClient struct {
	config RegistrationClientConfig
}

func NewRegistrationClient(config RegistrationClientConfig) *RegistrationClient {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = DefaultInitialBackoff
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = DefaultMaxBackoff
	}
	config.CoreURL = strings.TrimRight(config.CoreURL, "/")
	return &RegistrationClient{config: config}
}

func (client *RegistrationClient) Register(ctx context.Context, moduleManifest manifest.ModuleManifest) (RegisteredModule, error) {
	body, err := json.Marshal(moduleManifest.RegistrationRequest())
	if err != nil {
		return RegisteredModule{}, fmt.Errorf("encode registration: %w", err)
	}
	endpoint := client.config.CoreURL + "/internal/v1/modules/" + url.PathEscape(moduleManifest.ID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return RegisteredModule{}, fmt.Errorf("build registration request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.config.ModuleSecret)
	response, err := client.config.HTTPClient.Do(request)
	if err != nil {
		return RegisteredModule{}, fmt.Errorf("register module: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return RegisteredModule{}, &RejectedError{Status: response.StatusCode, Code: readErrorCode(response)}
	}
	if response.StatusCode != http.StatusOK {
		return RegisteredModule{}, fmt.Errorf("register module: core answered %d", response.StatusCode)
	}
	var registered RegisteredModule
	if err := json.NewDecoder(response.Body).Decode(&registered); err != nil {
		return RegisteredModule{}, fmt.Errorf("decode registration: %w", err)
	}
	return registered, nil
}

// RegisterUntilAccepted retries transient failures with exponential backoff and stops on a rejection or when ctx ends.
func (client *RegistrationClient) RegisterUntilAccepted(ctx context.Context, moduleManifest manifest.ModuleManifest) (RegisteredModule, error) {
	delay := client.config.InitialBackoff
	for {
		registered, err := client.Register(ctx, moduleManifest)
		if err == nil || errors.Is(err, ErrRegistrationRejected) {
			return registered, err
		}
		slog.WarnContext(ctx, "module registration failed, retrying", "module", moduleManifest.ID, "retryIn", delay, "err", err)
		select {
		case <-ctx.Done():
			return RegisteredModule{}, ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, client.config.MaxBackoff)
	}
}

func readErrorCode(response *http.Response) string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(response.Body).Decode(&body)
	return body.Error.Code
}
