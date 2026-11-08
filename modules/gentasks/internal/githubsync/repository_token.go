package githubsync

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const MaxTokenRepositories = 10

var (
	ErrGithubAppNotInstalled           = httpapi.NewAPIError(http.StatusConflict, "github_app_not_installed")
	ErrGithubAppPermissionsMissing     = httpapi.NewAPIError(http.StatusConflict, "github_app_permissions_missing")
	ErrGithubAppCredentialsUnreadable  = httpapi.NewAPIError(http.StatusConflict, "github_app_credentials_unreadable")
	ErrRepositoriesAcrossInstallations = httpapi.NewAPIError(http.StatusConflict, "github_repositories_across_installations")
	// ErrRepositoryTokensForBotsOnly: people push with their own GitHub accounts; only bot members get App tokens.
	ErrRepositoryTokensForBotsOnly = httpapi.NewAPIError(http.StatusForbidden, "repository_tokens_for_bots_only")
)

// TokenCaller is who asks for a token: its member token, and its access token to read core's directory with.
type TokenCaller struct {
	UserToken   string
	AccessToken string
}

var repositoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// RepositoryTokenPermissions is what a bot needs to push a branch, open a pull request, and post a review.
var RepositoryTokenPermissions = map[string]string{"contents": "write", "pull_requests": "write", "metadata": "read"}

// IssueRepositoryTokenRequest names the "owner/name" repositories a token is for.
type IssueRepositoryTokenRequest struct {
	Repositories []string `json:"repositories"`
}

func (request IssueRepositoryTokenRequest) Validate() ([]string, error) {
	seen := map[string]bool{}
	repositories := make([]string, 0, len(request.Repositories))
	for _, repository := range request.Repositories {
		repository = strings.TrimSpace(repository)
		if !repositoryNamePattern.MatchString(repository) {
			return nil, httpapi.NewValidationError(map[string]string{"repositories": "invalid"})
		}
		if key := strings.ToLower(repository); !seen[key] {
			seen[key] = true
			repositories = append(repositories, repository)
		}
	}
	if len(repositories) == 0 || len(repositories) > MaxTokenRepositories {
		return nil, httpapi.NewValidationError(map[string]string{"repositories": "invalid"})
	}
	return repositories, nil
}

// repositoryName is the part after the owner, which is how GitHub scopes an installation token.
func repositoryName(repository string) string {
	_, name, _ := strings.Cut(repository, "/")
	return name
}
