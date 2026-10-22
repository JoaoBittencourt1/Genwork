package remote_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/command"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/remote"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const syncInterval = 5 * time.Minute

type manualClock struct {
	mutex   sync.Mutex
	current time.Time
}

func (clock *manualClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.current
}

func (clock *manualClock) Advance(duration time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.current = clock.current.Add(duration)
}

// remoteFixture serves the HTTP MCP server against a fake core (JWKS) and a fake gateway (the Genwork APIs).
type remoteFixture struct {
	core   *coretest.FakeCore
	api    *apitest.FakeAPI
	clock  *manualClock
	server *httptest.Server
	// bearer is what the client sends next; tests change it to play a refresh or another user.
	bearer atomic.Value
}

func newRemoteFixture(t *testing.T) *remoteFixture {
	t.Helper()
	fixture := &remoteFixture{core: coretest.NewFakeCore(t), api: apitest.NewFakeAPI(t), clock: &manualClock{current: time.Now()}}
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/capabilities", http.StatusOK, "capabilities_with_gentasks.json")
	fixture.api.RespondWithFixture(t, http.MethodGet, "/api/core/v1/me", http.StatusOK, "me_admin.json")
	handler := remote.NewHandler(remote.HTTPServerConfig{
		PublicURL: coretest.Issuer, CoreInternalURL: fixture.core.URL(), GatewayInternalURL: fixture.api.URL(),
	}, remote.HandlerDependencies{
		Version: "test", HTTPClient: http.DefaultClient, Toolsets: command.Toolsets, SyncInterval: syncInterval, Now: fixture.clock.Now,
	})
	fixture.server = httptest.NewServer(handler)
	t.Cleanup(fixture.server.Close)
	fixture.useToken(fixture.core.MintAccessToken(t, "usr_admin"))
	return fixture
}

func (fixture *remoteFixture) useToken(token string) {
	fixture.bearer.Store(token)
}

type bearerTransport struct {
	fixture *remoteFixture
}

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+transport.fixture.bearer.Load().(string))
	return http.DefaultTransport.RoundTrip(request)
}

func (fixture *remoteFixture) connect(t *testing.T, onToolListChanged func()) *mcp.ClientSession {
	t.Helper()
	options := &mcp.ClientOptions{}
	if onToolListChanged != nil {
		options.ToolListChangedHandler = func(context.Context, *mcp.ToolListChangedRequest) { onToolListChanged() }
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, options)
	transport := &mcp.StreamableClientTransport{
		Endpoint: fixture.server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{fixture: fixture}}, MaxRetries: -1,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}
