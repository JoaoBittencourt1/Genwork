// Package toolsettest connects an in-memory MCP client to a server under test.
package toolsettest

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type StaticAccessReader struct {
	mutex    sync.Mutex
	snapshot toolset.AccessSnapshot
	Err      error
}

func NewStaticAccessReader(snapshot toolset.AccessSnapshot) *StaticAccessReader {
	return &StaticAccessReader{snapshot: snapshot}
}

func (reader *StaticAccessReader) Grant(snapshot toolset.AccessSnapshot) {
	reader.mutex.Lock()
	defer reader.mutex.Unlock()
	reader.snapshot = snapshot
}

func (reader *StaticAccessReader) ReadAccess(context.Context) (toolset.AccessSnapshot, error) {
	reader.mutex.Lock()
	defer reader.mutex.Unlock()
	return reader.snapshot, reader.Err
}

// FullAccess enables every module of the toolsets and grants every permission their tools need.
func FullAccess(toolsets ...toolset.Toolset) toolset.AccessSnapshot {
	var snapshot toolset.AccessSnapshot
	for _, set := range toolsets {
		if set.ModuleID != "" {
			snapshot.EnabledModuleIDs = append(snapshot.EnabledModuleIDs, set.ModuleID)
		}
		for _, tool := range set.Tools {
			if tool.Permission != "" && !slices.Contains(snapshot.Permissions, tool.Permission) {
				snapshot.Permissions = append(snapshot.Permissions, tool.Permission)
			}
		}
	}
	return snapshot
}

// ConnectClient connects an in-memory client; onToolListChanged may be nil.
func ConnectClient(t testing.TB, server *mcp.Server, onToolListChanged func()) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	options := &mcp.ClientOptions{}
	if onToolListChanged != nil {
		options.ToolListChangedHandler = func(context.Context, *mcp.ToolListChangedRequest) { onToolListChanged() }
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, options)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	})
	return clientSession
}

// ConnectWithFullAccess serves the toolsets with every module enabled and every permission granted.
func ConnectWithFullAccess(t testing.TB, toolsets ...toolset.Toolset) *mcp.ClientSession {
	t.Helper()
	server := toolset.NewServer("test")
	synchronizer := toolset.NewToolsetSynchronizer(server, NewStaticAccessReader(FullAccess(toolsets...)), toolsets...)
	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("sync toolsets: %v", err)
	}
	return ConnectClient(t, server, nil)
}

func ListToolNames(t testing.TB, session *mcp.ClientSession) []string {
	t.Helper()
	var names []string
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names
}

func CallTool(t testing.TB, session *mcp.ClientSession, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("call tool %s: %v", name, err)
	}
	return result
}

// ResultText answers the text of the first content item, where tool output and tool errors land.
func ResultText(t testing.TB, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool result content is %T, not text", result.Content[0])
	}
	return text.Text
}
