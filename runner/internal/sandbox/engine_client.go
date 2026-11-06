package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const engineBaseURL = "http://container-engine"

// EngineError is a refusal from the container engine.
type EngineError struct {
	Status  int
	Message string
}

func (err *EngineError) Error() string {
	return fmt.Sprintf("container engine answered %d: %s", err.Status, err.Message)
}

// engineClient speaks the Docker Engine API, which Podman also serves, over the local socket. It lives in the runner
// process; the socket is never mounted into a sandbox.
type engineClient struct {
	httpClient *http.Client
}

func newEngineClient(socketPath string) *engineClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &engineClient{httpClient: &http.Client{Transport: transport}}
}

// FindSocket picks the engine socket. A socket configured on purpose is the only one tried, so a typo never falls back to
// another engine; without one, DOCKER_HOST and then the well-known locations are tried.
func FindSocket(configured string, getenv func(string) string) (string, error) {
	if configured != "" {
		if info, err := os.Stat(configured); err == nil && info.Mode()&os.ModeSocket != 0 {
			return configured, nil
		}
		return "", ErrNoEngine
	}
	candidates := []string{}
	if host, found := strings.CutPrefix(getenv("DOCKER_HOST"), "unix://"); found {
		candidates = append(candidates, host)
	}
	if runtimeDir := getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, runtimeDir+"/podman/podman.sock", runtimeDir+"/docker.sock")
	}
	candidates = append(candidates, "/var/run/docker.sock", "/run/podman/podman.sock")
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.Mode()&os.ModeSocket != 0 {
			return candidate, nil
		}
	}
	return "", ErrNoEngine
}

func (client *engineClient) do(ctx context.Context, method, path string, query url.Values, body any, target any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode engine request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	address := engineBaseURL + path
	if len(query) > 0 {
		address += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, address, reader)
	if err != nil {
		return 0, fmt.Errorf("build engine request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrNoEngine, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return response.StatusCode, fmt.Errorf("read engine response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return response.StatusCode, &EngineError{Status: response.StatusCode, Message: strings.TrimSpace(string(payload))}
	}
	if target != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, target); err != nil {
			return response.StatusCode, fmt.Errorf("decode engine response: %w", err)
		}
	}
	return response.StatusCode, nil
}

// stream copies a response body to the writer until the engine closes it.
func (client *engineClient) stream(ctx context.Context, method, path string, query url.Values, output io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, method, engineBaseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build engine request: %w", err)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoEngine, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return &EngineError{Status: response.StatusCode, Message: strings.TrimSpace(string(message))}
	}
	_, err = io.Copy(output, response.Body)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stream engine output: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var engineErr *EngineError
	return errors.As(err, &engineErr) && engineErr.Status == http.StatusNotFound
}

const stopGracePeriod = 10 * time.Second
