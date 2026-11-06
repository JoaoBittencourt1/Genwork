package genagentstest

// TaskRun is a claimed dev run on acme/api with an Anthropic API-key account, shaped as genagents answers a claim.
func TaskRun(runToken string) map[string]any {
	return map[string]any{
		"runToken": runToken, "kind": "task", "bot": map[string]any{"name": "Dev Bot", "kind": "dev"},
		"prompt": "Work task GEN-1", "instructions": "Follow the conventions", "model": "claude-sonnet-5-5", "effort": "default",
		"maxTurns": 50, "timeoutMinutes": 5,
		"account":       map[string]any{"token": "acct-1", "provider": "anthropic", "mode": "api_key", "apiKey": "sk-ant-run-secret-0001"},
		"skills":        []any{map[string]any{"name": "go-conventions", "description": "How we write Go", "body": "# Go", "version": 1, "files": []any{}}},
		"mcpServers":    []any{},
		"genworkMcpUrl": "http://genwork.test/mcp", "resumeSession": nil,
		"repository": "acme/api", "branch": "bot/gen-1-abc123", "taskKey": "GEN-1", "taskTitle": "Speed up the API",
	}
}

// ReviewRun is a claimed review run over acme/api#12.
func ReviewRun(runToken string) map[string]any {
	run := TaskRun(runToken)
	run["kind"], run["bot"] = "review", map[string]any{"name": "Review Bot", "kind": "reviewer"}
	run["prompt"] = "Review GEN-1"
	delete(run, "repository")
	delete(run, "branch")
	run["pullRequests"] = []any{map[string]any{"repository": "acme/api", "number": 12, "url": "https://github.com/acme/api/pull/12", "title": "GEN-1 Speed up"}}
	return run
}

// ChatRun is a claimed admin chat turn: no repository.
func ChatRun(runToken string) map[string]any {
	run := TaskRun(runToken)
	run["kind"], run["bot"] = "chat", map[string]any{"name": "Ops Assistant", "kind": "assistant"}
	run["prompt"] = "list my tasks"
	delete(run, "repository")
	delete(run, "branch")
	return run
}
