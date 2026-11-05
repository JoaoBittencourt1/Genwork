// Package config holds the runner's settings: where the instance is, the runner's secret, and the limits of its sandboxes.
// The secret and the GitHub token live in one owner-only file on the runner's machine and nowhere else (rule 67).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultConcurrency    = 2
	DefaultSandboxCPUs    = 4
	DefaultSandboxMemory  = int64(8) << 30
	DefaultSandboxPIDs    = 1024
	configFileName        = "runner.json"
	ownerOnlyFileMode     = 0o600
	ownerOnlyDirectory    = 0o700
	environmentConfigDir  = "GENWORK_RUNNER_HOME"
	environmentInstance   = "GENWORK_INSTANCE_URL"
	environmentGithubTok  = "GH_TOKEN"
	environmentSandbox    = "GENWORK_SANDBOX_IMAGE"
	environmentConcurrent = "GENWORK_RUNNER_CONCURRENCY"
	environmentEngine     = "GENWORK_CONTAINER_SOCKET"
	environmentGitBase    = "GENWORK_GIT_BASE_URL"
	environmentGithubAPI  = "GITHUB_API_URL"
)

// Stored is what the config file keeps.
type Stored struct {
	InstanceURL  string `json:"instanceUrl"`
	RunnerToken  string `json:"runnerToken"`
	RunnerSecret string `json:"runnerSecret"`
	GithubToken  string `json:"githubToken,omitempty"`
}

// Limits are the resources a sandbox gets.
type Limits struct {
	CPUs   float64
	Memory int64
	PIDs   int64
}

type Config struct {
	Stored
	Home           string
	MaxConcurrency int
	SandboxImage   string
	CustomImage    bool
	DirectoryMode  bool
	EngineSocket   string
	GitBaseURL     string
	GithubAPIURL   string
	Limits         Limits
}

type Getenv func(string) string

// DefaultSandboxImage is the sandbox image of this runner's own version, which is the default.
func DefaultSandboxImage(registry, version string) string {
	return registry + "/genwork-sandbox:" + version
}

// Home resolves the directory holding the config file and the run workspaces.
func Home(getenv Getenv) (string, error) {
	if home := getenv(environmentConfigDir); home != "" {
		return home, nil
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find the runner home: %w", err)
	}
	return filepath.Join(userHome, ".genwork-runner"), nil
}

// Save writes the config file with owner-only permissions, creating its directory.
func Save(home string, stored Stored) error {
	if err := os.MkdirAll(home, ownerOnlyDirectory); err != nil {
		return fmt.Errorf("create the runner home: %w", err)
	}
	encoded, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the config: %w", err)
	}
	path := filepath.Join(home, configFileName)
	if err := os.WriteFile(path, encoded, ownerOnlyFileMode); err != nil {
		return fmt.Errorf("write the config: %w", err)
	}
	return os.Chmod(path, ownerOnlyFileMode)
}

// LoadStored reads only the config file, or an empty value when there is none yet.
func LoadStored(home string) (Stored, error) {
	payload, err := os.ReadFile(filepath.Join(home, configFileName))
	if errors.Is(err, os.ErrNotExist) {
		return Stored{}, nil
	}
	if err != nil {
		return Stored{}, fmt.Errorf("read the config: %w", err)
	}
	var stored Stored
	if err := json.Unmarshal(payload, &stored); err != nil {
		return Stored{}, fmt.Errorf("decode the config: %w", err)
	}
	return stored, nil
}

var ErrNotRegistered = errors.New("the runner is not registered: run `genwork-runner register` first")

// Load reads the config file and the environment. The instance URL has no default (rule 1): it comes from the file the
// registration wrote, or from GENWORK_INSTANCE_URL.
func Load(getenv Getenv, version, imageRegistry string) (Config, error) {
	home, err := Home(getenv)
	if err != nil {
		return Config{}, err
	}
	config := Config{Home: home, MaxConcurrency: DefaultConcurrency, EngineSocket: getenv(environmentEngine), GitBaseURL: getenv(environmentGitBase), GithubAPIURL: getenv(environmentGithubAPI)}
	payload, err := os.ReadFile(filepath.Join(home, configFileName))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, ErrNotRegistered
	}
	if err != nil {
		return Config{}, fmt.Errorf("read the config: %w", err)
	}
	if err := json.Unmarshal(payload, &config.Stored); err != nil {
		return Config{}, fmt.Errorf("decode the config: %w", err)
	}
	if override := getenv(environmentInstance); override != "" {
		config.InstanceURL = override
	}
	if token := getenv(environmentGithubTok); token != "" {
		config.GithubToken = token
	}
	config.InstanceURL = strings.TrimRight(config.InstanceURL, "/")
	if config.InstanceURL == "" || config.RunnerSecret == "" {
		return Config{}, ErrNotRegistered
	}
	if config.GitBaseURL == "" {
		config.GitBaseURL = "https://github.com"
	}
	if config.GithubAPIURL == "" {
		config.GithubAPIURL = "https://api.github.com"
	}
	if raw := getenv(environmentConcurrent); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 64 {
			return Config{}, fmt.Errorf("%s must be between 1 and 64", environmentConcurrent)
		}
		config.MaxConcurrency = parsed
	}
	config.SandboxImage = getenv(environmentSandbox)
	config.CustomImage = config.SandboxImage != ""
	if config.SandboxImage == "" && imageRegistry != "" {
		config.SandboxImage = DefaultSandboxImage(imageRegistry, version)
	}
	config.Limits = Limits{CPUs: DefaultSandboxCPUs, Memory: DefaultSandboxMemory, PIDs: DefaultSandboxPIDs}
	return config, nil
}

// WorkspacesDir is where each run's workspace lives, one directory per run token.
func (config Config) WorkspacesDir() string {
	return filepath.Join(config.Home, "runs")
}

// ProfilesDir holds one isolated sign-in profile directory per subscription account.
func (config Config) ProfilesDir() string {
	return filepath.Join(config.Home, "profiles")
}

// ProfileDir is the sign-in profile of one account; two accounts never share a directory.
func (config Config) ProfileDir(accountToken string) string {
	return filepath.Join(config.ProfilesDir(), filepath.Base(accountToken))
}

// HasGithubCredentials reports presence only; the heartbeat never carries the token itself.
func (config Config) HasGithubCredentials() bool {
	return config.GithubToken != ""
}
