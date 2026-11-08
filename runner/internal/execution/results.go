package execution

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExtractJSONObject returns the last JSON object in the text that has the given key, which is how a model's final answer
// carries its structured result among prose.
func ExtractJSONObject(text, requiredKey string) (map[string]any, bool) {
	var found map[string]any
	for index := 0; index < len(text); index++ {
		if text[index] != '{' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(text[index:]))
		var candidate map[string]any
		if err := decoder.Decode(&candidate); err != nil {
			continue
		}
		if _, has := candidate[requiredKey]; has {
			found = candidate
		}
	}
	return found, found != nil
}

// PullRequestDraft is what a dev run ends with.
type PullRequestDraft struct {
	Title       string
	Description string
}

// ParsePullRequestDraft reads the dev run's final JSON; the title must cite the task key, which is added when missing.
func ParsePullRequestDraft(finalText, taskKey string) (PullRequestDraft, bool) {
	object, found := ExtractJSONObject(finalText, "pullRequestTitle")
	if !found {
		return PullRequestDraft{}, false
	}
	title, _ := object["pullRequestTitle"].(string)
	description, _ := object["pullRequestBody"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		return PullRequestDraft{}, false
	}
	if taskKey != "" && !strings.Contains(strings.ToUpper(title), strings.ToUpper(taskKey)) {
		title = taskKey + " " + title
	}
	return PullRequestDraft{Title: title, Description: strings.TrimSpace(description)}, true
}

type Finding struct {
	Path     string
	Line     int
	Severity string
	Message  string
}

type PullRequestReview struct {
	Repository string
	Number     int
	Summary    string
	Findings   []Finding
}

var severities = map[string]bool{"info": true, "warning": true, "error": true}

// ParseReview validates the reviewer's JSON before anything is posted: every listed pull request must be one that was
// asked for, and every finding needs a path, a line, a known severity, and a message.
func ParseReview(finalText string, expected map[string]bool) ([]PullRequestReview, error) {
	object, found := ExtractJSONObject(finalText, "pullRequests")
	if !found {
		return nil, fmt.Errorf("the review has no pullRequests object")
	}
	listed, _ := object["pullRequests"].([]any)
	reviews := make([]PullRequestReview, 0, len(listed))
	for _, item := range listed {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("a pull request entry is not an object")
		}
		review, err := parsePullRequestReview(entry, expected)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	if len(reviews) == 0 {
		return nil, fmt.Errorf("the review lists no pull request")
	}
	return reviews, nil
}

func parsePullRequestReview(entry map[string]any, expected map[string]bool) (PullRequestReview, error) {
	repository, _ := entry["repository"].(string)
	number, _ := entry["number"].(float64)
	summary, _ := entry["summary"].(string)
	if !expected[PullRequestKey(repository, int(number))] {
		return PullRequestReview{}, fmt.Errorf("the review names %s#%d, which was not asked for", repository, int(number))
	}
	review := PullRequestReview{Repository: repository, Number: int(number), Summary: strings.TrimSpace(summary)}
	findings, _ := entry["findings"].([]any)
	for _, raw := range findings {
		finding, err := parseFinding(raw)
		if err != nil {
			return PullRequestReview{}, fmt.Errorf("%s#%d: %w", repository, int(number), err)
		}
		review.Findings = append(review.Findings, finding)
	}
	return review, nil
}

func parseFinding(raw any) (Finding, error) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return Finding{}, fmt.Errorf("a finding is not an object")
	}
	path, _ := entry["path"].(string)
	line, _ := entry["line"].(float64)
	severity, _ := entry["severity"].(string)
	message, _ := entry["message"].(string)
	if strings.TrimSpace(path) == "" || int(line) < 1 || !severities[severity] || strings.TrimSpace(message) == "" {
		return Finding{}, fmt.Errorf("a finding needs a path, a line, a severity of info, warning or error, and a message")
	}
	return Finding{Path: path, Line: int(line), Severity: severity, Message: strings.TrimSpace(message)}, nil
}

func PullRequestKey(repository string, number int) string {
	return fmt.Sprintf("%s#%d", repository, number)
}

// RenderReview is the full text of a review, which goes to the conversation when GitHub refuses it.
func RenderReview(reviews []PullRequestReview) string {
	var text strings.Builder
	for _, review := range reviews {
		fmt.Fprintf(&text, "%s#%d: %s\n", review.Repository, review.Number, review.Summary)
		for _, finding := range review.Findings {
			fmt.Fprintf(&text, "- [%s] %s:%d %s\n", finding.Severity, finding.Path, finding.Line, finding.Message)
		}
	}
	return text.String()
}

// SplitFindingsByDiff separates the findings GitHub accepts as inline comments, on a line in the pull request's diff,
// from the others. Without the diff (nil), every finding is tried inline.
func SplitFindingsByDiff(findings []Finding, commentable map[string]map[int]bool) (inline, outside []Finding) {
	for _, finding := range findings {
		if commentable == nil || commentable[finding.Path][finding.Line] {
			inline = append(inline, finding)
		} else {
			outside = append(outside, finding)
		}
	}
	return inline, outside
}

func findingText(finding Finding) string {
	return fmt.Sprintf("**%s**: %s", strings.ToUpper(finding.Severity), finding.Message)
}

// ReviewComments turns findings into inline review comments.
func ReviewComments(findings []Finding) []ReviewComment {
	comments := make([]ReviewComment, 0, len(findings))
	for _, finding := range findings {
		comments = append(comments, ReviewComment{Path: finding.Path, Line: finding.Line, Body: findingText(finding)})
	}
	return comments
}

// ReviewBody is the review's summary followed by the findings that cannot be inline comments, each with its location.
func ReviewBody(summary string, outside []Finding) string {
	if len(outside) == 0 {
		return summary
	}
	var body strings.Builder
	body.WriteString(summary)
	body.WriteString("\n\n")
	for _, finding := range outside {
		fmt.Fprintf(&body, "- `%s:%d` %s\n", finding.Path, finding.Line, findingText(finding))
	}
	return strings.TrimRight(body.String(), "\n")
}
