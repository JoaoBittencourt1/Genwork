// Package gentaskstest stubs gentasks' task context and assigned-tasks endpoints for genagents' tests, shaped by the
// recorded fixtures in sdk/go/gentaskstest (rules 25 and 62).
package gentaskstest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/gentaskstest"
)

type FakeGentasks struct {
	server    *httptest.Server
	mutex     sync.Mutex
	tasks     map[string]map[string]any
	assigned  map[string]time.Time
	forbidden map[string]bool
	hidden    map[string]bool
	down      bool
	// tokenRefusal is the error code the repository-token route answers with; empty issues the recorded token.
	tokenRefusal  string
	tokenRequests []RepositoryTokenRequest
}

// RepositoryTokenRequest records who asked gentasks for a GitHub token, and for which repositories.
type RepositoryTokenRequest struct {
	Bearer       string
	Repositories []string
}

func NewFakeGentasks(t testing.TB) *FakeGentasks {
	t.Helper()
	fake := &FakeGentasks{tasks: map[string]map[string]any{}, assigned: map[string]time.Time{}, forbidden: map[string]bool{}, hidden: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/gentasks/v1/tasks/{token}/context", fake.serveContext)
	mux.HandleFunc("GET /api/gentasks/v1/me/assigned-tasks", fake.serveAssigned)
	mux.HandleFunc("POST /api/gentasks/v1/github/repository-tokens", fake.serveRepositoryToken)
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *FakeGentasks) URL() string { return fake.server.URL }

// SetTask stores a task context built from the recorded fixture; mutate shapes it for the case at hand.
func (fake *FakeGentasks) SetTask(token string, mutate func(map[string]any)) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	var task map[string]any
	_ = json.Unmarshal(gentaskstest.Fixture("task_context.json"), &task)
	task["token"], task["key"] = token, "GEN-"+strings.ToUpper(token[len(token)-1:])
	if mutate != nil {
		mutate(task)
	}
	fake.tasks[token] = task
}

// Assign points a task at a member, as gentasks does when its assignee changes.
func (fake *FakeGentasks) Assign(taskToken string, memberToken *string, kind string, at time.Time) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	task := fake.tasks[taskToken]
	if memberToken == nil {
		task["assigneeToken"], task["assigneeKind"] = nil, nil
		return
	}
	task["assigneeToken"], task["assigneeKind"] = *memberToken, kind
	fake.assigned[taskToken] = at
}

// HideFromAssignedList leaves the task out of the assigned-tasks list while it stays assigned, as a task in a done column.
func (fake *FakeGentasks) HideFromAssignedList(taskToken string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.hidden[taskToken] = true
}

func (fake *FakeGentasks) Forbid(taskToken string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.forbidden[taskToken] = true
}

func (fake *FakeGentasks) GoDown() {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.down = true
}

func (fake *FakeGentasks) serveContext(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	token := r.PathValue("token")
	task, found := fake.tasks[token]
	switch {
	case fake.forbidden[token]:
		write(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "forbidden"}})
	case !found:
		write(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "task_not_found"}})
	default:
		write(w, http.StatusOK, task)
	}
}

// serveAssigned answers with the tasks assigned to the member the bearer token names ("bot-access-<member>-…").
func (fake *FakeGentasks) serveAssigned(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	tasks := []map[string]any{}
	for token, task := range fake.tasks {
		member, _ := task["assigneeToken"].(string)
		if member != "" && !fake.hidden[token] && strings.Contains(bearer, member) {
			var listed map[string]any
			var recorded struct {
				Tasks []map[string]any `json:"tasks"`
			}
			_ = json.Unmarshal(gentaskstest.Fixture("assigned_tasks.json"), &recorded)
			listed = recorded.Tasks[0]
			listed["token"], listed["assigneeToken"] = token, member
			listed["assignedAt"] = fake.assigned[token].Format(time.RFC3339)
			tasks = append(tasks, listed)
		}
	}
	write(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// RefuseRepositoryTokens makes gentasks refuse GitHub tokens with the error code, as when the App is not connected.
func (fake *FakeGentasks) RefuseRepositoryTokens(code string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.tokenRefusal = code
}

func (fake *FakeGentasks) RepositoryTokenRequests() []RepositoryTokenRequest {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return append([]RepositoryTokenRequest(nil), fake.tokenRequests...)
}

// serveRepositoryToken answers with the recorded token, or with the configured refusal.
func (fake *FakeGentasks) serveRepositoryToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repositories []string `json:"repositories"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	fake.tokenRequests = append(fake.tokenRequests, RepositoryTokenRequest{Bearer: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), Repositories: body.Repositories})
	if fake.tokenRefusal != "" {
		write(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": fake.tokenRefusal}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(gentaskstest.Fixture("repository_token.json"))
}
