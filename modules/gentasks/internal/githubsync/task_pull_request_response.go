package githubsync

type PullRequestAuthorResponse struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
}

// TaskPullRequestResponse: title, state, and author are null for pull requests linked before snapshots existed.
type TaskPullRequestResponse struct {
	Token      string                     `json:"token"`
	Repository string                     `json:"repository"`
	Number     int                        `json:"number"`
	Title      *string                    `json:"title"`
	State      *PullRequestState          `json:"state"`
	Author     *PullRequestAuthorResponse `json:"author"`
	URL        string                     `json:"url"`
}

// NewTaskPullRequestResponses never returns nil, so a task without pull requests serializes as an empty list.
func NewTaskPullRequestResponses(linked []LinkedPullRequest, githubWebURL string) []TaskPullRequestResponse {
	responses := make([]TaskPullRequestResponse, 0, len(linked))
	for _, link := range linked {
		pullRequest := link.PullRequest
		response := TaskPullRequestResponse{
			Token: pullRequest.Token, Repository: pullRequest.Repository, Number: pullRequest.Number, Title: pullRequest.Title,
			State: pullRequest.State, URL: PullRequestURL(githubWebURL, pullRequest.Repository, pullRequest.Number),
		}
		if pullRequest.Author != nil {
			response.Author = &PullRequestAuthorResponse{Login: pullRequest.Author.Login, AvatarURL: pullRequest.Author.AvatarURL}
		}
		responses = append(responses, response)
	}
	return responses
}
