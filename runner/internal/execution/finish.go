package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
)

// finish ends the run by its kind once the model stopped and no message is waiting. It returns finished when the run is
// over; otherwise the run goes on with the returned prompt, which is a person's reply to a question.
func (current *execution) finish(ctx context.Context, outcome adapters.Outcome, session string) (string, bool) {
	switch current.run.Kind {
	case "review":
		return current.finishReview(ctx, outcome, session)
	case "task":
		return current.finishTask(ctx, outcome, session)
	}
	current.poster.Post(ctx, adapters.EventResult, map[string]any{"summary": outcome.FinalText, "sessionId": session})
	return "", true
}

// finishTask opens the pull request when the bot committed, and treats an answer with neither a pull request draft nor
// commits as a question: the run waits for a person's reply.
func (current *execution) finishTask(ctx context.Context, outcome adapters.Outcome, session string) (string, bool) {
	repositoryDir := current.repositories[current.run.Repository]
	commits, err := current.workspace.CommitsAhead(ctx, repositoryDir)
	if err != nil {
		current.fail(ctx, reasonAdapterError, err.Error(), nil)
		return "", true
	}
	draft, hasDraft := ParsePullRequestDraft(outcome.FinalText, current.run.TaskKey)
	switch {
	case commits == 0 && !hasDraft:
		return current.waitForReply(ctx, session)
	case commits == 0:
		current.poster.Post(ctx, adapters.EventResult, map[string]any{"summary": outcome.FinalText, "sessionId": session, "noChanges": true})
		return "", true
	}
	if !hasDraft {
		draft = PullRequestDraft{Title: current.run.TaskKey + " " + current.run.TaskTitle, Description: outcome.FinalText}
	}
	if err := current.workspace.Push(ctx, current.run.Repository, repositoryDir, current.branch); err != nil {
		current.fail(ctx, reasonGithubPostFailed, err.Error(), nil)
		return "", true
	}
	base, err := current.githubPoster().DefaultBranch(ctx, current.run.Repository)
	if err != nil {
		current.fail(ctx, reasonGithubPostFailed, err.Error(), nil)
		return "", true
	}
	created, err := current.githubPoster().CreatePullRequest(ctx, current.run.Repository, current.branch, base, draft.Title, draft.Description)
	if err != nil {
		current.fail(ctx, reasonGithubPostFailed, err.Error(), nil)
		return "", true
	}
	current.poster.Post(ctx, adapters.EventResult, map[string]any{
		"summary": outcome.FinalText, "sessionId": session, "pullRequestUrl": created.URL, "pullRequestNumber": created.Number, "repository": current.run.Repository,
	})
	return "", true
}

// waitForReply reports the run as waiting for input and polls for a person's message until one arrives or the run ends.
func (current *execution) waitForReply(ctx context.Context, session string) (string, bool) {
	current.poster.Post(ctx, adapters.EventStatus, map[string]any{"state": "waiting_for_input", "sessionId": session})
	ticker := time.NewTicker(current.executor.settings.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", true
		case <-ticker.C:
			if reply, found := current.pendingMessages(ctx); found {
				current.poster.Post(ctx, adapters.EventStatus, map[string]any{"state": "running", "sessionId": session})
				return reply, false
			}
		}
	}
}

// ErrReviewContentRejected is GitHub refusing a review's content (422), typically a comment on a line outside the diff.
var ErrReviewContentRejected = errors.New("github refused the review content")

// reviewResendPrompt asks the model, once, for the review JSON again when its answer could not be read.
const reviewResendPrompt = "Your review could not be read: %s. Reply with only the corrected JSON object " +
	`{"pullRequests": [{"repository", "number", "summary", "findings": [{"path", "line", "severity", "message"}]}]}` +
	", valid and complete, with nothing before or after it."

// finishReview validates the reviewer's findings, then posts one GitHub review per pull request. An unreadable answer
// gets one resend request; when GitHub still refuses, the full review text goes to the conversation and the run fails
// with github_post_failed.
func (current *execution) finishReview(ctx context.Context, outcome adapters.Outcome, session string) (string, bool) {
	expected := map[string]bool{}
	for _, pullRequest := range current.run.PullRequests {
		expected[PullRequestKey(pullRequest.Repository, pullRequest.Number)] = true
	}
	reviews, err := ParseReview(outcome.FinalText, expected)
	if err != nil && !current.reviewRetried {
		current.reviewRetried = true
		return fmt.Sprintf(reviewResendPrompt, err.Error()), false
	}
	if err != nil {
		current.fail(ctx, reasonAdapterError, "the review could not be read: "+err.Error(), nil)
		return "", true
	}
	posted := make([]map[string]any, 0, len(reviews))
	total := 0
	for _, review := range reviews {
		url, err := current.postReview(ctx, review)
		if err != nil {
			current.poster.Post(ctx, adapters.EventAssistantMessage, map[string]any{"text": adapters.Redact(RenderReview(reviews), current.secrets...)})
			current.fail(ctx, reasonGithubPostFailed, err.Error(), nil)
			return "", true
		}
		total += len(review.Findings)
		posted = append(posted, map[string]any{"repository": review.Repository, "number": review.Number, "findings": len(review.Findings), "url": url})
	}
	current.poster.Post(ctx, adapters.EventResult, map[string]any{
		"summary": fmt.Sprintf("%d findings", total), "sessionId": session, "findings": total, "reviews": posted,
	})
	return "", true
}

// postReview comments inline on the findings GitHub accepts there, the lines in the diff, and lists the rest in the
// review body; when GitHub still refuses the inline comments, it posts every finding in the body.
func (current *execution) postReview(ctx context.Context, review PullRequestReview) (string, error) {
	commentable, err := current.githubPoster().CommentableLines(ctx, review.Repository, review.Number)
	if err != nil {
		commentable = nil
	}
	inline, outside := SplitFindingsByDiff(review.Findings, commentable)
	url, err := current.githubPoster().CreateReview(ctx, review.Repository, review.Number, ReviewBody(review.Summary, outside), ReviewComments(inline))
	if errors.Is(err, ErrReviewContentRejected) && len(inline) > 0 {
		return current.githubPoster().CreateReview(ctx, review.Repository, review.Number, ReviewBody(review.Summary, review.Findings), nil)
	}
	return url, err
}
