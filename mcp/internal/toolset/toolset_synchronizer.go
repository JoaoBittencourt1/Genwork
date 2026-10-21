package toolset

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Toolset groups the tools of core (empty ModuleID, always enabled) or of one module.
type Toolset struct {
	ModuleID string
	Tools    []Tool
}

// AccessSnapshot is what the signed-in user can reach right now: the enabled modules and their own permissions.
type AccessSnapshot struct {
	EnabledModuleIDs []string
	Permissions      []string
}

type AccessReader interface {
	ReadAccess(ctx context.Context) (AccessSnapshot, error)
}

// ToolsetSynchronizer exposes only the tools of enabled modules that the user may call (rule 58). Adding or
// removing a tool makes the MCP server notify clients with tools/list_changed.
type ToolsetSynchronizer struct {
	server     *mcp.Server
	toolsets   []Toolset
	access     AccessReader
	mutex      sync.Mutex
	registered map[string]bool
}

func NewToolsetSynchronizer(server *mcp.Server, access AccessReader, toolsets ...Toolset) *ToolsetSynchronizer {
	return &ToolsetSynchronizer{server: server, toolsets: toolsets, access: access, registered: map[string]bool{}}
}

func (synchronizer *ToolsetSynchronizer) Sync(ctx context.Context) error {
	snapshot, err := synchronizer.access.ReadAccess(ctx)
	if err != nil {
		return err
	}
	allowed := AllowedTools(snapshot, synchronizer.toolsets)
	synchronizer.mutex.Lock()
	defer synchronizer.mutex.Unlock()
	synchronizer.removeToolsNotIn(allowed)
	synchronizer.addMissingTools(allowed)
	return nil
}

// SyncEvery keeps the tools in step with modules enabled, disabled, or permissions changed while the server runs.
func (synchronizer *ToolsetSynchronizer) SyncEvery(ctx context.Context, interval time.Duration, reportError func(error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := synchronizer.Sync(ctx); err != nil && ctx.Err() == nil {
				reportError(err)
			}
		}
	}
}

func (synchronizer *ToolsetSynchronizer) RegisteredToolNames() []string {
	synchronizer.mutex.Lock()
	defer synchronizer.mutex.Unlock()
	names := make([]string, 0, len(synchronizer.registered))
	for name := range synchronizer.registered {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (synchronizer *ToolsetSynchronizer) removeToolsNotIn(allowed map[string]Tool) {
	var stale []string
	for name := range synchronizer.registered {
		if _, ok := allowed[name]; !ok {
			stale = append(stale, name)
			delete(synchronizer.registered, name)
		}
	}
	if len(stale) > 0 {
		synchronizer.server.RemoveTools(stale...)
	}
}

func (synchronizer *ToolsetSynchronizer) addMissingTools(allowed map[string]Tool) {
	for name, tool := range allowed {
		if !synchronizer.registered[name] {
			tool.addTo(synchronizer.server)
			synchronizer.registered[name] = true
		}
	}
}

func AllowedTools(snapshot AccessSnapshot, toolsets []Toolset) map[string]Tool {
	allowed := map[string]Tool{}
	for _, toolset := range toolsets {
		if toolset.ModuleID != "" && !slices.Contains(snapshot.EnabledModuleIDs, toolset.ModuleID) {
			continue
		}
		for _, tool := range toolset.Tools {
			if tool.Permission == "" || slices.Contains(snapshot.Permissions, tool.Permission) {
				allowed[tool.Definition.Name] = tool
			}
		}
	}
	return allowed
}
