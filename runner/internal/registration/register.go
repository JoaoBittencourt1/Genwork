// Package registration exchanges a registration token for the runner's secret and stores it on this machine.
package registration

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
)

var ErrMissingInput = errors.New("register needs the instance URL and the registration token")

// Register exchanges the token and writes the config file with owner-only permissions. It keeps any GitHub token
// already stored, so registering again after a revocation does not lose it.
func Register(ctx context.Context, home, instanceURL, registrationToken string, httpClient *http.Client) error {
	instanceURL = strings.TrimRight(strings.TrimSpace(instanceURL), "/")
	registrationToken = strings.TrimSpace(registrationToken)
	if instanceURL == "" || registrationToken == "" {
		return ErrMissingInput
	}
	runnerToken, secret, err := api.Register(ctx, instanceURL, registrationToken, httpClient)
	if err != nil {
		return err
	}
	previous, err := config.LoadStored(home)
	if err != nil {
		return err
	}
	return config.Save(home, config.Stored{InstanceURL: instanceURL, RunnerToken: runnerToken, RunnerSecret: secret, GithubToken: previous.GithubToken})
}
