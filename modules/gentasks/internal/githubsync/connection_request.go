package githubsync

import (
	"regexp"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	maxGithubLoginLength  = 39
	maxManifestCodeLength = 100
)

var (
	// githubLoginPattern: letters and digits, with single hyphens only between them, as GitHub requires.
	githubLoginPattern  = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)
	manifestCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type StartManifestRequest struct {
	Organization *string `json:"organization"`
}

// StartManifestInput.Organization is empty when the App belongs to the admin's personal account.
type StartManifestInput struct {
	Organization string
}

func (request StartManifestRequest) Validate() (StartManifestInput, error) {
	if request.Organization == nil {
		return StartManifestInput{}, nil
	}
	organization := strings.TrimSpace(*request.Organization)
	if organization != "" && (len(organization) > maxGithubLoginLength || !githubLoginPattern.MatchString(organization)) {
		return StartManifestInput{}, httpapi.NewValidationError(map[string]string{"organization": "invalid"})
	}
	return StartManifestInput{Organization: organization}, nil
}

type CompleteConnectionRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type CompleteConnectionInput struct {
	Code  string
	State string
}

func (request CompleteConnectionRequest) Validate() (CompleteConnectionInput, error) {
	fields := map[string]string{}
	input := CompleteConnectionInput{Code: strings.TrimSpace(request.Code), State: strings.TrimSpace(request.State)}
	if len(input.Code) > maxManifestCodeLength || !manifestCodePattern.MatchString(input.Code) {
		fields["code"] = "invalid"
	}
	if input.State == "" {
		fields["state"] = "invalid"
	}
	if len(fields) > 0 {
		return CompleteConnectionInput{}, httpapi.NewValidationError(fields)
	}
	return input, nil
}
