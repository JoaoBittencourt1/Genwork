package runs_test

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

func TestRepositoriesListsEachRepositoryOnce(t *testing.T) {
	spec := runs.RunSpec{Repository: "acme/api", PullRequests: []runs.PullRequestRef{
		{Repository: "acme/web", Number: 1}, {Repository: "acme/api", Number: 2}, {Repository: "acme/web", Number: 3},
	}}

	if got := strings.Join(spec.Repositories(), ","); got != "acme/api,acme/web" {
		t.Errorf("repositories = %s", got)
	}
	if got := (runs.RunSpec{}).Repositories(); len(got) != 0 {
		t.Errorf("an empty spec has repositories: %v", got)
	}
}

func TestGithubCredentialsMissingIsAKnownFailure(t *testing.T) {
	if !runs.FailureGithubCredentialsMissing.IsValid() {
		t.Error("github_credentials_missing is not a valid failure reason")
	}
}
