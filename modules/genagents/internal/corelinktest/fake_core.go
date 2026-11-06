// Package corelinktest stubs core's bot member, token, and delegated session endpoints for genagents' tests, shaped by
// the recorded contract fixtures in sdk/go/coretest (rules 25 and 62).
package corelinktest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
)

type Member struct {
	Token  string
	Name   string
	Role   string
	Active bool
	Secret string
}

type FakeCore struct {
	server   *httptest.Server
	mutex    sync.Mutex
	members  map[string]*Member
	refusal  *refusal
	sessions map[string]*session
	counter  int
	calls    []string
	now      func() time.Time
}

type refusal struct {
	status int
	code   string
}

type session struct {
	user    string
	revoked bool
	current string
	access  string
}

func NewFakeCore(t testing.TB, now func() time.Time) *FakeCore {
	t.Helper()
	core := &FakeCore{members: map[string]*Member{}, sessions: map[string]*session{}, now: now}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/core/v1/bots", core.createBot)
	mux.HandleFunc("PATCH /api/core/v1/bots/{token}", core.patchBot)
	mux.HandleFunc("DELETE /api/core/v1/bots/{token}", core.deleteBot)
	mux.HandleFunc("POST /api/core/v1/bots/{token}/secret", core.rotateSecret)
	mux.HandleFunc("POST /api/core/v1/oauth/token", core.issueToken)
	mux.HandleFunc("POST /api/core/v1/delegated-sessions", core.grantSession)
	mux.HandleFunc("POST /api/core/v1/auth/refresh", core.refreshSession)
	mux.HandleFunc("POST /api/core/v1/auth/logout", core.logout)
	core.server = httptest.NewServer(mux)
	t.Cleanup(core.server.Close)
	return core
}

func (core *FakeCore) URL() string { return core.server.URL }

// RefuseNext makes the next bot member or session call fail with core's error shape.
func (core *FakeCore) RefuseNext(status int, code string) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.refusal = &refusal{status: status, code: code}
}

func (core *FakeCore) Calls() []string {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	return append([]string(nil), core.calls...)
}

func (core *FakeCore) MemberByName(name string) (Member, bool) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	for _, member := range core.members {
		if member.Name == name {
			return *member, true
		}
	}
	return Member{}, false
}

func (core *FakeCore) Member(token string) (Member, bool) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	member, found := core.members[token]
	if !found {
		return Member{}, false
	}
	return *member, true
}

// RevokeDelegatedSessions ends every delegated session, as the admin would from their session list.
func (core *FakeCore) RevokeDelegatedSessions() {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	for _, delegated := range core.sessions {
		delegated.revoked = true
	}
}

func (core *FakeCore) record(r *http.Request) {
	core.calls = append(core.calls, r.Method+" "+r.URL.Path)
}

func (core *FakeCore) nextID(prefix string) string {
	core.counter++
	return fmt.Sprintf("%s-%d", prefix, core.counter)
}

// takeRefusal answers a pending refusal and reports whether it did.
func (core *FakeCore) takeRefusal(w http.ResponseWriter) bool {
	if core.refusal == nil {
		return false
	}
	refused := core.refusal
	core.refusal = nil
	writeJSON(w, refused.status, map[string]any{"error": map[string]any{"code": refused.code}})
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeBody(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body
}

func memberBody(member *Member, secret string) map[string]any {
	var body map[string]any
	_ = json.Unmarshal(coretest.Fixture("bot_member_created.json"), &body)
	body["token"], body["name"] = member.Token, member.Name
	body["role"] = map[string]any{"token": member.Role, "name": "member"}
	if member.Active {
		body["status"] = "active"
	} else {
		body["status"] = "deactivated"
	}
	if secret != "" {
		body["clientSecret"] = secret
	} else {
		delete(body, "clientSecret")
	}
	return body
}

func (core *FakeCore) createBot(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	if core.takeRefusal(w) {
		return
	}
	body := decodeBody(r)
	member := &Member{Token: core.nextID("bot-member"), Name: fmt.Sprint(body["name"]), Role: fmt.Sprint(body["role"]), Active: true, Secret: core.nextID("bot-secret")}
	core.members[member.Token] = member
	writeJSON(w, http.StatusCreated, memberBody(member, member.Secret))
}

func (core *FakeCore) patchBot(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	if core.takeRefusal(w) {
		return
	}
	member, found := core.members[r.PathValue("token")]
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "member_not_found"}})
		return
	}
	body := decodeBody(r)
	if name, present := body["name"].(string); present {
		member.Name = name
	}
	if role, present := body["role"].(string); present {
		member.Role = role
	}
	if status, present := body["status"].(string); present {
		member.Active = status == "active"
	}
	writeJSON(w, http.StatusOK, memberBody(member, ""))
}

func (core *FakeCore) deleteBot(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	if core.takeRefusal(w) {
		return
	}
	delete(core.members, r.PathValue("token"))
	w.WriteHeader(http.StatusNoContent)
}

func (core *FakeCore) rotateSecret(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	if core.takeRefusal(w) {
		return
	}
	member, found := core.members[r.PathValue("token")]
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"code": "member_not_found"}})
		return
	}
	member.Secret = core.nextID("bot-secret")
	writeJSON(w, http.StatusOK, memberBody(member, member.Secret))
}

func (core *FakeCore) issueToken(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	token, secret, found := r.BasicAuth()
	member := core.members[token]
	if !found || member == nil || !member.Active || member.Secret != secret {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client", "error_description": "the client credentials are invalid"})
		return
	}
	var body map[string]any
	_ = json.Unmarshal(coretest.Fixture("bot_access_token.json"), &body)
	body["access_token"] = "bot-access-" + member.Token + "-" + core.nextID("n")
	writeJSON(w, http.StatusOK, body)
}

func bearerOf(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func (core *FakeCore) sessionBody(delegated *session, sessionToken, refresh string) map[string]any {
	var body map[string]any
	_ = json.Unmarshal(coretest.Fixture("delegated_session.json"), &body)
	delegated.access = "delegated-access-" + core.nextID("n")
	delegated.current = refresh
	body["accessToken"], body["refreshToken"], body["session"] = delegated.access, refresh, sessionToken
	body["accessTokenExpiresAt"] = core.now().Add(15 * time.Minute).Format(time.RFC3339)
	return body
}

func (core *FakeCore) grantSession(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	if core.takeRefusal(w) {
		return
	}
	sessionToken := core.nextID("session")
	delegated := &session{user: bearerOf(r)}
	core.sessions[sessionToken] = delegated
	writeJSON(w, http.StatusOK, core.sessionBody(delegated, sessionToken, core.nextID("refresh")))
}

func (core *FakeCore) refreshSession(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	refresh, _ := decodeBody(r)["refreshToken"].(string)
	for sessionToken, delegated := range core.sessions {
		if delegated.current == refresh && !delegated.revoked {
			writeJSON(w, http.StatusOK, core.sessionBody(delegated, sessionToken, core.nextID("refresh")))
			return
		}
	}
	writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"code": "session_revoked"}})
}

func (core *FakeCore) logout(w http.ResponseWriter, r *http.Request) {
	core.mutex.Lock()
	defer core.mutex.Unlock()
	core.record(r)
	for _, delegated := range core.sessions {
		if delegated.access == bearerOf(r) {
			delegated.revoked = true
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
