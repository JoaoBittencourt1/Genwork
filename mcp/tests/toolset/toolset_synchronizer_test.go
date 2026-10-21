package toolset_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoInput struct {
	Text string `json:"text"`
}

func echoTool(name, permission string) toolset.Tool {
	return toolset.NewTool(&mcp.Tool{Name: name, Description: name}, permission, nil,
		func(_ context.Context, input echoInput) (any, error) {
			return map[string]string{"echo": input.Text}, nil
		})
}

func sampleToolsets() []toolset.Toolset {
	return []toolset.Toolset{
		{Tools: []toolset.Tool{echoTool("core_open", ""), echoTool("core_guarded", "core.thing.read")}},
		{ModuleID: "notes", Tools: []toolset.Tool{echoTool("notes_read", "notes.note.read"), echoTool("notes_open", "")}},
	}
}

func TestSyncExposesOnlyEnabledModulesAndGrantedPermissions(t *testing.T) {
	reader := toolsettest.NewStaticAccessReader(toolset.AccessSnapshot{Permissions: []string{"notes.note.read"}})
	server := toolset.NewServer("test")
	synchronizer := toolset.NewToolsetSynchronizer(server, reader, sampleToolsets()...)

	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	names := toolsettest.ListToolNames(t, toolsettest.ConnectClient(t, server, nil))
	if !slices.Equal(names, []string{"core_open"}) {
		t.Fatalf("tools = %v, want only the open core tool while notes is disabled", names)
	}
}

func TestSyncFollowsModulesAndPermissionsChangingAtRuntime(t *testing.T) {
	reader := toolsettest.NewStaticAccessReader(toolset.AccessSnapshot{})
	server := toolset.NewServer("test")
	synchronizer := toolset.NewToolsetSynchronizer(server, reader, sampleToolsets()...)
	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	changed := make(chan struct{}, 8)
	session := toolsettest.ConnectClient(t, server, func() { changed <- struct{}{} })

	reader.Grant(toolset.AccessSnapshot{EnabledModuleIDs: []string{"notes"}, Permissions: []string{"notes.note.read", "core.thing.read"}})
	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync after enabling: %v", err)
	}
	waitForToolListChanged(t, changed)
	if names := toolsettest.ListToolNames(t, session); !slices.Equal(names, []string{"core_guarded", "core_open", "notes_open", "notes_read"}) {
		t.Fatalf("tools after enabling = %v", names)
	}

	reader.Grant(toolset.AccessSnapshot{Permissions: []string{"core.thing.read"}})
	if err := synchronizer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync after disabling: %v", err)
	}
	waitForToolListChanged(t, changed)
	if names := synchronizer.RegisteredToolNames(); !slices.Equal(names, []string{"core_guarded", "core_open"}) {
		t.Fatalf("tools after disabling = %v", names)
	}
}

func TestSyncKeepsTheCurrentToolsWhenAccessCannotBeRead(t *testing.T) {
	reader := toolsettest.NewStaticAccessReader(toolset.AccessSnapshot{})
	synchronizer := toolset.NewToolsetSynchronizer(toolset.NewServer("test"), reader, sampleToolsets()...)
	_ = synchronizer.Sync(context.Background())
	reader.Err = errors.New("instance down")

	if err := synchronizer.Sync(context.Background()); !errors.Is(err, reader.Err) {
		t.Fatalf("err = %v", err)
	}
	if names := synchronizer.RegisteredToolNames(); !slices.Equal(names, []string{"core_open"}) {
		t.Fatalf("tools = %v", names)
	}
}

func TestSyncEveryReportsErrorsUntilCanceled(t *testing.T) {
	reader := toolsettest.NewStaticAccessReader(toolset.AccessSnapshot{})
	reader.Err = errors.New("instance down")
	synchronizer := toolset.NewToolsetSynchronizer(toolset.NewServer("test"), reader, sampleToolsets()...)
	ctx, cancel := context.WithCancel(context.Background())
	reported := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		synchronizer.SyncEvery(ctx, time.Millisecond, func(err error) {
			select {
			case reported <- err:
			default:
			}
		})
		close(done)
	}()

	if err := <-reported; !errors.Is(err, reader.Err) {
		t.Fatalf("reported %v", err)
	}
	cancel()
	<-done
}

func TestAllowedToolsTreatsAnEmptyPermissionAsOpenToEverySignedInUser(t *testing.T) {
	allowed := toolset.AllowedTools(toolset.AccessSnapshot{EnabledModuleIDs: []string{"notes"}}, sampleToolsets())

	if _, ok := allowed["notes_open"]; !ok || len(allowed) != 2 {
		t.Fatalf("allowed = %v", allowed)
	}
}

func waitForToolListChanged(t *testing.T, changed <-chan struct{}) {
	t.Helper()
	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("the client was not notified that the tool list changed")
	}
}
