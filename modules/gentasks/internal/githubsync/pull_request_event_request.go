package githubsync

import (
	"encoding/json"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// PullRequestEventRequest decodes only the fields gentasks reads; GitHub sends many more, so unknown fields are allowed.
type PullRequestEventRequest struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		Draft     bool      `json:"draft"`
		Merged    bool      `json:"merged"`
		UpdatedAt time.Time `json:"updated_at"`
		User      struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func DecodePullRequestEvent(body []byte) (PullRequestEvent, error) {
	var request PullRequestEventRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return PullRequestEvent{}, httpapi.ErrMalformedBody
	}
	if request.PullRequest.Number <= 0 || request.Repository.FullName == "" || request.PullRequest.UpdatedAt.IsZero() {
		return PullRequestEvent{}, httpapi.ErrMalformedBody
	}
	return PullRequestEvent{
		Action:     PullRequestAction(request.Action),
		Repository: request.Repository.FullName,
		Number:     request.PullRequest.Number,
		Title:      request.PullRequest.Title,
		HeadRef:    request.PullRequest.Head.Ref,
		BaseRef:    request.PullRequest.Base.Ref,
		Merged:     request.PullRequest.Merged,
		State:      PullRequestStateOf(request.PullRequest.State, request.PullRequest.Draft, request.PullRequest.Merged),
		Author:     PullRequestAuthor{Login: request.PullRequest.User.Login, AvatarURL: request.PullRequest.User.AvatarURL},
		UpdatedAt:  request.PullRequest.UpdatedAt,
	}, nil
}
