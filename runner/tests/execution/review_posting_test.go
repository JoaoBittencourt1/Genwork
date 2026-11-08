package execution_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/execution"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const twoFindingsReview = `{"pullRequests": [{"repository": "acme/api", "number": 12, "summary": "FIX FIRST", "findings": [` +
	`{"path": "orders.go", "line": 3, "severity": "warning", "message": "missing nil check"},` +
	`{"path": "model.go", "line": 160, "severity": "error", "message": "field never filled"}]}]}`

func reviewEnvironment(t *testing.T, answers ...string) *environment {
	t.Helper()
	env := newEnvironment(t, func(_ context.Context, turn int, _ sandbox.Spec, output io.Writer) (sandbox.Result, error) {
		answer := answers[len(answers)-1]
		if turn <= len(answers) {
			answer = answers[turn-1]
		}
		_, err := io.WriteString(output, resultLine(answer))
		return sandbox.Result{}, err
	}, nil)
	env.remote.AddPullRequestHead(t, 12, "feature.txt", "x")
	return env
}

func TestFindingsOutsideTheDiffGoToTheReviewBody(t *testing.T) {
	env := reviewEnvironment(t, twoFindingsReview)
	env.github.Commentable = map[string]map[int]bool{"orders.go": {3: true}}

	env.execute(genagentstest.ReviewRun("run-1"))

	if len(env.github.Reviews) != 1 {
		t.Fatalf("reviews = %v", env.github.Reviews)
	}
	review := env.github.Reviews[0]
	comments := review["comments"].([]execution.ReviewComment)
	body := review["summary"].(string)
	if len(comments) != 1 || comments[0].Path != "orders.go" || comments[0].Line != 3 {
		t.Errorf("inline comments = %+v", comments)
	}
	if !strings.HasPrefix(body, "FIX FIRST") || !strings.Contains(body, "`model.go:160`") || !strings.Contains(body, "field never filled") {
		t.Errorf("body = %q", body)
	}
	if result := env.lastEvent("run-1"); result.Type != "result" || result.Payload["findings"].(float64) != 2 {
		t.Errorf("result = %+v", result)
	}
}

func TestAReviewGithubRefusesInlineIsPostedWithEveryFindingInTheBody(t *testing.T) {
	env := reviewEnvironment(t, twoFindingsReview)
	env.github.RejectInline = true

	env.execute(genagentstest.ReviewRun("run-1"))

	if len(env.github.Reviews) != 1 {
		t.Fatalf("reviews = %v", env.github.Reviews)
	}
	review := env.github.Reviews[0]
	body := review["summary"].(string)
	if len(review["comments"].([]execution.ReviewComment)) != 0 || !strings.Contains(body, "`orders.go:3`") || !strings.Contains(body, "`model.go:160`") {
		t.Errorf("review = %+v", review)
	}
	if result := env.lastEvent("run-1"); result.Type != "result" {
		t.Errorf("result = %+v", result)
	}
}

func TestAnUnreadableReviewIsAskedForOnceMore(t *testing.T) {
	broken := strings.TrimSuffix(twoFindingsReview, "]}")
	env := reviewEnvironment(t, "Done. "+broken, twoFindingsReview)

	env.execute(genagentstest.ReviewRun("run-1"))

	if env.launcher.Turns() != 2 {
		t.Fatalf("turns = %d", env.launcher.Turns())
	}
	resend := strings.Join(env.launcher.Specs[1].Command, " ")
	if !strings.Contains(resend, "could not be read") || !strings.Contains(resend, "--resume") {
		t.Errorf("the second turn does not ask for the JSON in the same session: %s", resend)
	}
	if len(env.github.Reviews) != 1 {
		t.Errorf("reviews = %v", env.github.Reviews)
	}
}
