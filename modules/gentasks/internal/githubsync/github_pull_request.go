package githubsync

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PullRequestState string

const (
	PullRequestOpen   PullRequestState = "open"
	PullRequestDraft  PullRequestState = "draft"
	PullRequestMerged PullRequestState = "merged"
	PullRequestClosed PullRequestState = "closed"
)

const githubClosedState = "closed"

// PullRequestStateOf folds GitHub's state, draft, and merged fields into the one state people see.
func PullRequestStateOf(githubState string, draft, merged bool) PullRequestState {
	switch {
	case merged:
		return PullRequestMerged
	case githubState == githubClosedState:
		return PullRequestClosed
	case draft:
		return PullRequestDraft
	default:
		return PullRequestOpen
	}
}

type PullRequestAuthor struct {
	Login     string
	AvatarURL string
}

// GithubPullRequest is gentasks' snapshot of a pull request. Title, State, Author, and GithubUpdatedAt are nil only for
// pull requests linked before snapshots existed, until their next delivery.
type GithubPullRequest struct {
	ID              int64
	Token           string
	Repository      string
	Number          int
	Title           *string
	State           *PullRequestState
	Author          *PullRequestAuthor
	GithubUpdatedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PullRequestSnapshot is what a delivery says about a pull request; GithubUpdatedAt orders deliveries.
type PullRequestSnapshot struct {
	Repository      string
	Number          int
	Title           string
	State           PullRequestState
	Author          PullRequestAuthor
	GithubUpdatedAt time.Time
}

// SavedPullRequest reports where a snapshot was stored; Current is false when a newer one was already stored.
type SavedPullRequest struct {
	ID      int64
	Current bool
}

// PullRequestURL is the pull request's page on the configured GitHub host; repository is "owner/name".
func PullRequestURL(githubWebURL, repository string, number int) string {
	owner, name, _ := strings.Cut(repository, "/")
	return strings.TrimSuffix(githubWebURL, "/") + "/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/pull/" + strconv.Itoa(number)
}
