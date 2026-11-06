package app_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/gentaskstest"
)

// shapeOf lists the key paths of a decoded JSON value; arrays contribute the keys of their first element, if any.
func shapeOf(prefix string, value any, paths map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			paths[prefix+"."+key] = true
			shapeOf(prefix+"."+key, nested, paths)
		}
	case []any:
		if len(typed) > 0 {
			shapeOf(prefix+"[]", typed[0], paths)
		}
	}
}

// sortedPaths leaves out the keys under the listed array prefixes, which the live task here has no element for.
func sortedPaths(value any, skipPrefixes ...string) []string {
	paths := map[string]bool{}
	shapeOf("", value, paths)
	listed := make([]string, 0, len(paths))
	for path := range paths {
		skipped := false
		for _, prefix := range skipPrefixes {
			skipped = skipped || strings.HasPrefix(path, prefix)
		}
		if !skipped {
			listed = append(listed, path)
		}
	}
	sort.Strings(listed)
	return listed
}

// TestResponsesMatchRecordedFixtures keeps the fixtures other modules stub gentasks with in step with its real API.
func TestResponsesMatchRecordedFixtures(t *testing.T) {
	app := newTestApp(t)
	app.addBotMember()
	board := app.createBoard("Genwork")
	app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "acme/api"})
	label := app.createLabel(board.Token, "backend", "blue")
	task := app.createTask(board.Token, map[string]any{"title": "Speed up", "assigneeToken": userToken, "labelTokens": []string{label.Token}})
	live := map[string]response{
		"task_context.json":   app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + task.Token + "/context"}),
		"assigned_tasks.json": app.do(request{method: "GET", path: assignedTasksPath}),
	}

	for _, name := range gentaskstest.FixtureNames {
		t.Run(name, func(t *testing.T) {
			var recorded, current any
			if err := json.Unmarshal(gentaskstest.Fixture(name), &recorded); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			live[name].decode(t, &current)
			if want, got := fmt.Sprint(sortedPaths(recorded, ".pullRequests[]")), fmt.Sprint(sortedPaths(current, ".pullRequests[]")); want != got {
				t.Errorf("%s: recorded keys %s, live keys %s", name, want, got)
			}
		})
	}
}
