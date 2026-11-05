package taskruns

import (
	"fmt"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
)

// PromptComposer builds the prompt of each run kind from a fixed template and the task context. genagents owns the
// templates, so changing one never needs a new runner (design §8).
type PromptComposer struct{}

const taskPromptTemplate = `You are working on the Genwork task %s: %s

Board: %s
Repository: %s (already checked out for you on branch %s)
%s%s
Task description:
%s

How to work:
- Use the Genwork MCP tools to report progress and to move the task along the board as you go.
- Implement the task inside the repository, run the checks you can find in it (tests, linters, builds), and fix what they report.
- Commit your changes locally on the current branch. Do not push, do not open a pull request, and never merge: the runner pushes your branch and opens the pull request after you finish.
- If you cannot continue without a decision from a person, ask your question in your reply and stop.
- Finish with a short summary and a JSON object on its own line: {"pullRequestTitle": "<title that starts with %s>", "pullRequestBody": "<what changed and how you checked it>"}.
`

// ComposeTaskPrompt describes the task and how a dev bot works it.
func (PromptComposer) ComposeTaskPrompt(task gentasksclient.TaskContext, repository, branch string) string {
	description := "(no description)"
	if task.Description != nil && strings.TrimSpace(*task.Description) != "" {
		description = *task.Description
	}
	return fmt.Sprintf(taskPromptTemplate, task.Key, task.Title, task.Board.Name, repository, branch,
		labelsLine(task), linkedPullRequestsLine(task.PullRequests), description, task.Key)
}

const reviewPromptTemplate = `You are reviewing the pull requests linked to the Genwork task %s: %s

Task description (the acceptance context):
%s

Pull requests to review:
%s
For each pull request, read its diff against its base branch and check: correctness, whether the change does what the task asks, tests, and the repository's own conventions files when present (for example a constitution or contributor guide).
Do not change any file. Do not approve or request changes: the runner posts your findings as comments.

Finish with one JSON object on its own line, with exactly this shape:
{"pullRequests": [{"repository": "owner/name", "number": 1, "summary": "<overall assessment>", "findings": [{"path": "<file>", "line": 1, "severity": "info|warning|error", "message": "<what is wrong and why>"}]}]}
Include every pull request listed above, with an empty findings list when you find nothing.
`

// ComposeReviewPrompt asks for structured findings the runner validates before posting them.
func (PromptComposer) ComposeReviewPrompt(task gentasksclient.TaskContext, reviewable []gentasksclient.PullRequest) string {
	description := "(no description)"
	if task.Description != nil && strings.TrimSpace(*task.Description) != "" {
		description = *task.Description
	}
	var listed strings.Builder
	for _, pullRequest := range reviewable {
		fmt.Fprintf(&listed, "- %s#%d %s\n", pullRequest.Repository, pullRequest.Number, pullRequest.URL)
	}
	return fmt.Sprintf(reviewPromptTemplate, task.Key, task.Title, description, listed.String())
}

func labelsLine(task gentasksclient.TaskContext) string {
	if len(task.Labels) == 0 {
		return ""
	}
	names := make([]string, 0, len(task.Labels))
	for _, label := range task.Labels {
		names = append(names, label.Name)
	}
	return "Labels: " + strings.Join(names, ", ") + "\n"
}

func linkedPullRequestsLine(pullRequests []gentasksclient.PullRequest) string {
	if len(pullRequests) == 0 {
		return ""
	}
	listed := make([]string, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		listed = append(listed, fmt.Sprintf("%s#%d", pullRequest.Repository, pullRequest.Number))
	}
	return "Linked pull requests: " + strings.Join(listed, ", ") + "\n"
}
