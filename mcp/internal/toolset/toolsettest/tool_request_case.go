package toolsettest

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
)

// ToolRequestCase is one tool call and the API request it must send. A non-empty Confirmation means the API
// answers 204 and the tool must report that confirmation; otherwise the tool must answer the API body as is.
type ToolRequestCase struct {
	Tool         string
	Arguments    map[string]any
	Method       string
	Path         string
	Query        string
	Body         any
	Confirmation string
}

func (testCase ToolRequestCase) Name() string {
	if testCase.Query == "" {
		return testCase.Tool
	}
	return testCase.Tool + "?" + testCase.Query
}

const answeredFixture = "boards.json"

func NewFakeAPIRequester(t testing.TB) (*apitest.FakeAPI, platform.APIRequester) {
	api := apitest.NewFakeAPI(t)
	tokens := apitest.NewStaticTokenSource("access")
	return api, platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), tokens)
}

func ExpectToolRequest(t *testing.T, newToolset func(platform.APIRequester) toolset.Toolset, testCase ToolRequestCase) {
	t.Helper()
	api, requester := NewFakeAPIRequester(t)
	if testCase.Confirmation == "" {
		api.RespondWithFixture(t, testCase.Method, testCase.Path, http.StatusOK, answeredFixture)
	} else {
		api.RespondEmpty(testCase.Method, testCase.Path, http.StatusNoContent)
	}
	session := ConnectWithFullAccess(t, newToolset(requester))

	result := CallTool(t, session, testCase.Tool, testCase.Arguments)

	if result.IsError {
		t.Fatalf("tool error: %s", ResultText(t, result))
	}
	expectSentRequest(t, api.LastRequest(t), testCase)
	expectToolOutput(t, ResultText(t, result), testCase)
}

func expectSentRequest(t *testing.T, request apitest.RecordedRequest, testCase ToolRequestCase) {
	t.Helper()
	if request.Method != testCase.Method || request.Path != testCase.Path || request.Query.Encode() != testCase.Query {
		t.Fatalf("sent %s %s?%s, want %s %s?%s", request.Method, request.Path, request.Query.Encode(), testCase.Method, testCase.Path, testCase.Query)
	}
	if body := request.DecodedBody(t); !reflect.DeepEqual(body, normalizeJSON(t, testCase.Body)) {
		t.Fatalf("sent body %s, want %v", request.Body, testCase.Body)
	}
}

func expectToolOutput(t *testing.T, text string, testCase ToolRequestCase) {
	t.Helper()
	want := strings.TrimSpace(string(apitest.Fixture(t, answeredFixture)))
	if testCase.Confirmation != "" {
		want = `{"result":"` + testCase.Confirmation + `"}`
	}
	if !reflect.DeepEqual(normalizeJSON(t, json.RawMessage(text)), normalizeJSON(t, json.RawMessage(want))) {
		t.Fatalf("tool output %s, want %s", text, want)
	}
}

func normalizeJSON(t testing.TB, value any) any {
	t.Helper()
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode %v: %v", value, err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode %s: %v", encoded, err)
	}
	return decoded
}
