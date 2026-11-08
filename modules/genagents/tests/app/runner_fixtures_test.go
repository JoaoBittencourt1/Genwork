package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
)

type testRunner struct {
	Token  string
	Secret string
	Name   string
}

type heartbeatOptions struct {
	tools          []string
	github         bool
	directoryMode  bool
	maxConcurrency int
}

func (app *testApp) createRunnerViaAPI(name string) (runnerBody, string) {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/genagents/v1/runners", body: map[string]any{"name": name}})
	expectStatus(app.t, resp, http.StatusCreated)
	var created struct {
		runnerBody
		RegistrationToken string `json:"registrationToken"`
	}
	resp.decode(app.t, &created)
	return created.runnerBody, created.RegistrationToken
}

type runnerBody struct {
	Token                 string            `json:"token"`
	Name                  string            `json:"name"`
	Online                bool              `json:"online"`
	Revoked               bool              `json:"revoked"`
	Registered            bool              `json:"registered"`
	DirectoryMode         bool              `json:"directoryMode"`
	HasGithubCredentials  bool              `json:"hasGithubCredentials"`
	CustomImage           bool              `json:"customImage"`
	ContainerEngine       *string           `json:"containerEngine"`
	SandboxImage          *string           `json:"sandboxImage"`
	Tools                 map[string]string `json:"tools"`
	RegistrationExpiresAt *time.Time        `json:"registrationExpiresAt"`
	CleanupPending        bool              `json:"cleanupPending"`
	LastCleanupAt         *time.Time        `json:"lastCleanupAt"`
	LastCleanupFreedBytes *int64            `json:"lastCleanupFreedBytes"`
}

func (app *testApp) removeRunner(token string) response {
	app.t.Helper()
	return app.do(request{method: "POST", path: "/api/genagents/v1/runners/" + token + "/removal"})
}

func (app *testApp) runnerNames() []string {
	app.t.Helper()
	names := make([]string, 0)
	for _, runner := range app.listRunnerBodies() {
		names = append(names, runner.Name)
	}
	return names
}

func (app *testApp) runnerByToken(token string) runnerBody {
	app.t.Helper()
	for _, runner := range app.listRunnerBodies() {
		if runner.Token == token {
			return runner
		}
	}
	app.t.Fatalf("runner %s not listed", token)
	return runnerBody{}
}

func (app *testApp) listRunnerBodies() []runnerBody {
	app.t.Helper()
	var body struct {
		Runners []runnerBody `json:"runners"`
	}
	app.do(request{method: "GET", path: "/api/genagents/v1/runners"}).decode(app.t, &body)
	return body.Runners
}

func (app *testApp) runnerDo(secret, method, path string, body any) response {
	app.t.Helper()
	return app.do(request{method: method, path: path, body: body, bearer: secret})
}

func (app *testApp) sendHeartbeat(runner testRunner, options heartbeatOptions) response {
	app.t.Helper()
	tools := map[string]string{}
	for _, tool := range options.tools {
		tools[tool] = "1.0.0"
	}
	concurrency := options.maxConcurrency
	if concurrency == 0 {
		concurrency = 2
	}
	return app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/heartbeat", map[string]any{
		"containerEngine": "docker", "sandboxImage": "genwork-sandbox:test", "directoryMode": options.directoryMode, "tools": tools,
		"hasGithubCredentials": options.github, "maxConcurrency": concurrency,
	})
}

// registerRunner creates a runner through the admin API, exchanges its registration token, and sends a first heartbeat.
func (app *testApp) registerRunner(name string, options heartbeatOptions) testRunner {
	app.t.Helper()
	created, registrationToken := app.createRunnerViaAPI(name)
	resp := app.do(request{method: "POST", path: "/api/genagents/v1/runner/register", anonymous: true, body: map[string]any{"registrationToken": registrationToken}})
	expectStatus(app.t, resp, http.StatusOK)
	var registered struct {
		RunnerToken string `json:"runnerToken"`
		Secret      string `json:"secret"`
	}
	resp.decode(app.t, &registered)
	runner := testRunner{Token: created.Token, Secret: registered.Secret, Name: name}
	expectStatus(app.t, app.sendHeartbeat(runner, options), http.StatusOK)
	return runner
}

func (app *testApp) botID(botToken string) int64 {
	app.t.Helper()
	var id int64
	if err := app.db.QueryRow(context.Background(), `SELECT id FROM bots WHERE token = $1`, botToken).Scan(&id); err != nil {
		app.t.Fatalf("find bot: %v", err)
	}
	return id
}

// queueRun stores a queued run for a bot the way run creation does, with a conversation of its own.
func (app *testApp) queueRun(botToken string, kind runs.RunKind, taskToken string) runs.Run {
	app.t.Helper()
	ctx := context.Background()
	conversation, err := runs.ConversationRepository{}.FindOrCreateTask(ctx, app.db, taskToken, app.clock.Now())
	if err != nil {
		app.t.Fatalf("create conversation: %v", err)
	}
	run, err := runs.RunRepository{}.Create(ctx, app.db, runs.NewRun{
		ConversationID: conversation.ID, BotID: app.botID(botToken), Kind: kind, RequesterUserToken: adminUserToken,
		Spec: runs.RunSpec{Prompt: "Do the task", Repository: "acme/api", Branch: "bot/gen-1-abc", TaskKey: "GEN-1"}, At: app.clock.Now(),
	})
	if err != nil {
		app.t.Fatalf("create run: %v", err)
	}
	return run
}

type claimBody struct {
	RunToken       string `json:"runToken"`
	Kind           string `json:"kind"`
	Prompt         string `json:"prompt"`
	Instructions   string `json:"instructions"`
	Model          string `json:"model"`
	Repository     string `json:"repository"`
	Branch         string `json:"branch"`
	GenworkMCPURL  string `json:"genworkMcpUrl"`
	TimeoutMinutes int    `json:"timeoutMinutes"`
	Account        struct {
		Token    string `json:"token"`
		Provider string `json:"provider"`
		Mode     string `json:"mode"`
		APIKey   string `json:"apiKey"`
	} `json:"account"`
	Skills []struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
		Files   []struct {
			Path string `json:"path"`
		} `json:"files"`
	} `json:"skills"`
	MCPServers []struct {
		Name    string `json:"name"`
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"mcpServers"`
}

func (app *testApp) claim(runner testRunner, freeSlots int) (*claimBody, response) {
	app.t.Helper()
	resp := app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/claims", map[string]any{"freeSlots": freeSlots})
	if resp.status != http.StatusOK {
		return nil, resp
	}
	var claimed claimBody
	resp.decode(app.t, &claimed)
	return &claimed, resp
}

func (app *testApp) runState(runToken string) string {
	app.t.Helper()
	var state string
	if err := app.db.QueryRow(context.Background(), `SELECT state FROM runs WHERE token = $1`, runToken).Scan(&state); err != nil {
		app.t.Fatalf("read run state: %v", err)
	}
	return state
}

func (app *testApp) runFailureReason(runToken string) string {
	app.t.Helper()
	var reason *string
	if err := app.db.QueryRow(context.Background(), `SELECT failure_reason FROM runs WHERE token = $1`, runToken).Scan(&reason); err != nil {
		app.t.Fatalf("read failure reason: %v", err)
	}
	if reason == nil {
		return ""
	}
	return *reason
}

type eventBody struct {
	Sequence int64          `json:"sequence"`
	Type     string         `json:"type"`
	Payload  map[string]any `json:"payload"`
}

func (app *testApp) postEvents(runner testRunner, runToken string, events ...eventBody) response {
	app.t.Helper()
	return app.runnerDo(runner.Secret, "POST", "/api/genagents/v1/runner/runs/"+runToken+"/events", map[string]any{"events": events})
}

func (app *testApp) conversationEntries(runToken string) []map[string]any {
	app.t.Helper()
	rows, err := app.db.Query(context.Background(), `
		SELECT conversation_entries.type, conversation_entries.payload FROM conversation_entries
		JOIN runs ON runs.id = conversation_entries.run_id WHERE runs.token = $1 ORDER BY conversation_entries.sequence`, runToken)
	if err != nil {
		app.t.Fatalf("list entries: %v", err)
	}
	defer rows.Close()
	var entries []map[string]any
	for rows.Next() {
		var entryType string
		var payload []byte
		if err := rows.Scan(&entryType, &payload); err != nil {
			app.t.Fatalf("scan entry: %v", err)
		}
		decoded := map[string]any{}
		_ = json.Unmarshal(payload, &decoded)
		decoded["_type"] = entryType
		entries = append(entries, decoded)
	}
	return entries
}

func expectNoContent(t *testing.T, resp response) {
	t.Helper()
	expectStatus(t, resp, http.StatusNoContent)
}
