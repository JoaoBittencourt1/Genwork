package runexec

import (
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/mcpservers"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runs"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/skills"
)

type ClaimedAccountResponse struct {
	Token    string `json:"token"`
	Provider string `json:"provider"`
	Mode     string `json:"mode"`
	APIKey   string `json:"apiKey,omitempty"`
}

type ClaimedSkillFileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ClaimedSkillResponse struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Body        string                     `json:"body"`
	Version     int                        `json:"version"`
	Files       []ClaimedSkillFileResponse `json:"files"`
}

type ClaimedEntryResponse struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ClaimedMCPServerResponse struct {
	Name      string                 `json:"name"`
	Transport string                 `json:"transport"`
	URL       string                 `json:"url,omitempty"`
	Headers   []ClaimedEntryResponse `json:"headers,omitempty"`
	Command   string                 `json:"command,omitempty"`
	Args      []string               `json:"args,omitempty"`
	Env       []ClaimedEntryResponse `json:"env,omitempty"`
}

type ClaimedBotResponse struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// ClaimedRunResponse is the run specification a runner receives once: secrets included, which is why it is sent only on
// claim and never stored.
type ClaimedRunResponse struct {
	RunToken       string                     `json:"runToken"`
	Kind           runs.RunKind               `json:"kind"`
	Bot            ClaimedBotResponse         `json:"bot"`
	Prompt         string                     `json:"prompt"`
	Instructions   string                     `json:"instructions"`
	Model          string                     `json:"model"`
	Effort         string                     `json:"effort"`
	MaxTurns       int                        `json:"maxTurns"`
	TimeoutMinutes int                        `json:"timeoutMinutes"`
	Account        ClaimedAccountResponse     `json:"account"`
	Skills         []ClaimedSkillResponse     `json:"skills"`
	MCPServers     []ClaimedMCPServerResponse `json:"mcpServers"`
	GenworkMCPURL  string                     `json:"genworkMcpUrl"`
	ResumeSession  *string                    `json:"resumeSession"`
	Repository     string                     `json:"repository,omitempty"`
	Branch         string                     `json:"branch,omitempty"`
	TaskKey        string                     `json:"taskKey,omitempty"`
	TaskTitle      string                     `json:"taskTitle,omitempty"`
	PullRequests   []runs.PullRequestRef      `json:"pullRequests,omitempty"`
}

func NewClaimedRunResponse(claimed ClaimedRun, genworkMCPURL string) ClaimedRunResponse {
	instructions := ""
	if claimed.Bot.Instructions != nil {
		instructions = *claimed.Bot.Instructions
	}
	spec := claimed.Run.Spec
	return ClaimedRunResponse{
		RunToken: claimed.Run.Token, Kind: claimed.Run.Kind, Bot: ClaimedBotResponse{Name: claimed.Bot.Name, Kind: string(claimed.Bot.Kind)},
		Prompt: spec.Prompt, Instructions: instructions, Model: claimed.Bot.Model, Effort: string(claimed.Bot.Effort), MaxTurns: claimed.Bot.MaxTurns,
		TimeoutMinutes: claimed.Bot.TimeoutMinutes,
		Account: ClaimedAccountResponse{
			Token: claimed.Account.Token, Provider: string(claimed.Account.Provider), Mode: string(claimed.Account.Mode), APIKey: claimed.APIKey,
		},
		Skills: newClaimedSkills(claimed.Skills), MCPServers: newClaimedServers(claimed.MCPServers), GenworkMCPURL: genworkMCPURL,
		ResumeSession: claimed.Run.CLISessionID, Repository: spec.Repository, Branch: spec.Branch, TaskKey: spec.TaskKey, TaskTitle: spec.TaskTitle,
		PullRequests: spec.PullRequests,
	}
}

func newClaimedSkills(listed []skills.Skill) []ClaimedSkillResponse {
	responses := make([]ClaimedSkillResponse, 0, len(listed))
	for _, skill := range listed {
		files := make([]ClaimedSkillFileResponse, 0, len(skill.Files))
		for _, file := range skill.Files {
			files = append(files, ClaimedSkillFileResponse{Path: file.Path, Content: file.Content})
		}
		responses = append(responses, ClaimedSkillResponse{Name: skill.Name, Description: skill.Description, Body: skill.Body, Version: skill.Version, Files: files})
	}
	return responses
}

func newClaimedServers(listed []mcpservers.McpServer) []ClaimedMCPServerResponse {
	responses := make([]ClaimedMCPServerResponse, 0, len(listed))
	for _, server := range listed {
		response := ClaimedMCPServerResponse{
			Name: server.Name, Transport: string(server.Transport), URL: server.Config.URL, Command: server.Config.Command, Args: server.Config.Args,
			Headers: resolveEntries("header", server.Config.Headers, server.Secrets), Env: resolveEntries("env", server.Config.Env, server.Secrets),
		}
		responses = append(responses, response)
	}
	return responses
}

func resolveEntries(kind string, entries []mcpservers.Entry, secrets map[string]string) []ClaimedEntryResponse {
	resolved := make([]ClaimedEntryResponse, 0, len(entries))
	for _, entry := range entries {
		value := entry.Value
		if entry.Secret {
			value = secrets[mcpservers.SecretKey(kind, entry.Name)]
		}
		resolved = append(resolved, ClaimedEntryResponse{Name: entry.Name, Value: value})
	}
	return resolved
}
