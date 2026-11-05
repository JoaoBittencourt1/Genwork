// Package genagentstest stubs genagents' runner API for the runner's tests, with the same routes and answers (rules 25
// and 62).
package genagentstest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const Secret = "runner-secret-for-tests"

type PostedEvent struct {
	RunToken string
	Sequence int64
	Type     string
	Payload  map[string]any
}

type FakeGenagents struct {
	server *httptest.Server
	mutex  sync.Mutex
	// Heartbeats, Events, and Calls record what the runner sent.
	Heartbeats []map[string]any
	Events     []PostedEvent
	Calls      []string
	seen       map[string]bool
	claims     []map[string]any
	messages   map[string][]map[string]any
	states     map[string]string
	accounts   []map[string]string
	tokens     int
	revoked    bool
	failEvents int
	// Acked is the last acknowledged message sequence per run.
	Acked map[string]int64
	// AccountStatuses records ReportAccountStatus calls.
	AccountStatuses []map[string]any
	// RegistrationToken is the one token the register route accepts.
	RegistrationToken string
}

func NewFakeGenagents(t testing.TB) *FakeGenagents {
	t.Helper()
	fake := &FakeGenagents{seen: map[string]bool{}, messages: map[string][]map[string]any{}, states: map[string]string{}, Acked: map[string]int64{}, RegistrationToken: "registration-token"}
	mux := http.NewServeMux()
	base := "/api/genagents/v1/runner"
	mux.HandleFunc("POST "+base+"/register", fake.register)
	mux.HandleFunc("POST "+base+"/heartbeat", fake.authenticated(fake.heartbeat))
	mux.HandleFunc("POST "+base+"/claims", fake.authenticated(fake.claim))
	mux.HandleFunc("GET "+base+"/runs/{token}", fake.authenticated(fake.runState))
	mux.HandleFunc("POST "+base+"/runs/{token}/events", fake.authenticated(fake.postEvents))
	mux.HandleFunc("GET "+base+"/runs/{token}/messages", fake.authenticated(fake.listMessages))
	mux.HandleFunc("POST "+base+"/runs/{token}/messages/ack", fake.authenticated(fake.acknowledge))
	mux.HandleFunc("POST "+base+"/runs/{token}/genwork-token", fake.authenticated(fake.genworkToken))
	mux.HandleFunc("POST "+base+"/accounts/{token}/status", fake.authenticated(fake.accountStatus))
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func (fake *FakeGenagents) URL() string { return fake.server.URL }

// QueueRun makes the next claim return this run, built from the recorded shape of a claimed run.
func (fake *FakeGenagents) QueueRun(run map[string]any) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.claims = append(fake.claims, run)
	fake.states[run["runToken"].(string)] = "running"
}

func (fake *FakeGenagents) QueueMessage(runToken string, sequence int64, text string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.messages[runToken] = append(fake.messages[runToken], map[string]any{"sequence": sequence, "text": text})
}

func (fake *FakeGenagents) SetRunState(runToken, state string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.states[runToken] = state
}

// AssignAccount lists a subscription account the runner holds, as the heartbeat answer does.
func (fake *FakeGenagents) AssignAccount(token, provider string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.accounts = append(fake.accounts, map[string]string{"token": token, "provider": provider})
}

// Revoke makes every authenticated route answer 401.
func (fake *FakeGenagents) Revoke() {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.revoked = true
}

// FailNextEvents makes the next event posts answer 503 before one succeeds.
func (fake *FakeGenagents) FailNextEvents(count int) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.failEvents = count
}

func (fake *FakeGenagents) EventsOf(runToken string) []PostedEvent {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	var listed []PostedEvent
	for _, event := range fake.Events {
		if event.RunToken == runToken {
			listed = append(listed, event)
		}
	}
	return listed
}

func (fake *FakeGenagents) TokenRequests() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.tokens
}

func (fake *FakeGenagents) HeartbeatCount() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return len(fake.Heartbeats)
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func readBody(r *http.Request) map[string]any {
	var body map[string]any
	payload, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(payload, &body)
	return body
}

func (fake *FakeGenagents) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fake.mutex.Lock()
		fake.Calls = append(fake.Calls, r.Method+" "+r.URL.Path)
		revoked := fake.revoked
		fake.mutex.Unlock()
		if revoked || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != Secret {
			write(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "invalid_runner_secret"}})
			return
		}
		next(w, r)
	}
}

func (fake *FakeGenagents) register(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if body["registrationToken"] != fake.RegistrationToken || fake.seen["registered"] {
		write(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "invalid_registration_token"}})
		return
	}
	fake.seen["registered"] = true
	write(w, http.StatusOK, map[string]any{"runnerToken": "runner-token", "name": "build-box", "secret": Secret})
}

func (fake *FakeGenagents) heartbeat(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Heartbeats = append(fake.Heartbeats, body)
	accounts := fake.accounts
	if accounts == nil {
		accounts = []map[string]string{}
	}
	write(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (fake *FakeGenagents) claim(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if len(fake.claims) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	run := fake.claims[0]
	fake.claims = fake.claims[1:]
	write(w, http.StatusOK, run)
}

func (fake *FakeGenagents) runState(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	token := r.PathValue("token")
	write(w, http.StatusOK, map[string]any{"token": token, "state": fake.states[token]})
}

func (fake *FakeGenagents) postEvents(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	token := r.PathValue("token")
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	if fake.failEvents > 0 {
		fake.failEvents--
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	listed, _ := body["events"].([]any)
	for _, raw := range listed {
		event := raw.(map[string]any)
		sequence := int64(event["sequence"].(float64))
		key := token + "#" + strconv.FormatInt(sequence, 10)
		payload, _ := event["payload"].(map[string]any)
		if fake.seen[key] {
			continue
		}
		fake.seen[key] = true
		fake.Events = append(fake.Events, PostedEvent{RunToken: token, Sequence: sequence, Type: event["type"].(string), Payload: payload})
		fake.applyState(token, event["type"].(string), payload)
	}
	write(w, http.StatusOK, map[string]any{"accepted": len(listed), "state": fake.states[token]})
}

// applyState mirrors how genagents moves a run on the events it receives.
func (fake *FakeGenagents) applyState(token, eventType string, payload map[string]any) {
	if !isActive(fake.states[token]) {
		return
	}
	switch eventType {
	case "result":
		fake.states[token] = "succeeded"
	case "error":
		fake.states[token] = "failed"
	case "status":
		if state, _ := payload["state"].(string); state == "running" || state == "waiting_for_input" {
			fake.states[token] = state
		}
	}
}

func isActive(state string) bool {
	return state == "queued" || state == "running" || state == "waiting_for_input"
}

func (fake *FakeGenagents) listMessages(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	token := r.PathValue("token")
	pending := []map[string]any{}
	for _, message := range fake.messages[token] {
		if int64(message["sequence"].(int64)) > fake.Acked[token] {
			pending = append(pending, message)
		}
	}
	write(w, http.StatusOK, map[string]any{"messages": pending})
}

func (fake *FakeGenagents) acknowledge(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Acked[r.PathValue("token")] = int64(body["upToSequence"].(float64))
	w.WriteHeader(http.StatusNoContent)
}

func (fake *FakeGenagents) genworkToken(w http.ResponseWriter, r *http.Request) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.tokens++
	write(w, http.StatusOK, map[string]any{
		"accessToken": "genwork-access-token-" + string(rune('a'+fake.tokens)), "expiresAt": time.Now().Add(15 * time.Minute).Format(time.RFC3339),
		"mcpUrl": "http://genwork.test/mcp",
	})
}

func (fake *FakeGenagents) accountStatus(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	body["account"] = r.PathValue("token")
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.AccountStatuses = append(fake.AccountStatuses, body)
	w.WriteHeader(http.StatusNoContent)
}
