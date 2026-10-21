package toolset_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform/apitest"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset/toolsettest"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type colorInput struct {
	Color string  `json:"color"`
	Note  *string `json:"note,omitempty"`
}

func TestNewToolRestrictsEnumPropertiesAndMarksOptionalOnes(t *testing.T) {
	tool := toolset.NewTool(&mcp.Tool{Name: "paint"}, "", map[string][]any{"color": {"red", "blue"}},
		func(context.Context, colorInput) (any, error) { return nil, nil })

	schema := tool.Definition.InputSchema.(*jsonschema.Schema)
	if !slices.Equal(schema.Properties["color"].Enum, []any{"red", "blue"}) {
		t.Fatalf("color enum = %v", schema.Properties["color"].Enum)
	}
	if !slices.Equal(schema.Required, []string{"color"}) {
		t.Fatalf("required = %v", schema.Required)
	}
}

func TestNewToolPanicsOnAnEnumForAnUnknownProperty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewTool accepted an enum for a property the input does not have")
		}
	}()
	toolset.NewTool(&mcp.Tool{Name: "paint"}, "", map[string][]any{"shade": {"dark"}},
		func(context.Context, colorInput) (any, error) { return nil, nil })
}

func TestToolCallRejectsAValueOutsideTheEnumBeforeCallingTheAPI(t *testing.T) {
	called := false
	tool := toolset.NewTool(&mcp.Tool{Name: "paint"}, "", map[string][]any{"color": {"red"}},
		func(context.Context, colorInput) (any, error) { called = true; return nil, nil })
	session := toolsettest.ConnectWithFullAccess(t, toolset.Toolset{Tools: []toolset.Tool{tool}})

	result := toolsettest.CallTool(t, session, "paint", map[string]any{"color": "green"})

	if !result.IsError || called {
		t.Fatalf("isError = %v, called = %v", result.IsError, called)
	}
}

func TestToolCallErrorsBecomeToolResultsTheModelCanRead(t *testing.T) {
	tool := toolset.NewTool(&mcp.Tool{Name: "paint"}, "", nil,
		func(context.Context, colorInput) (any, error) {
			return nil, &platform.APIError{Status: http.StatusConflict, Code: "label_name_taken"}
		})
	session := toolsettest.ConnectWithFullAccess(t, toolset.Toolset{Tools: []toolset.Tool{tool}})

	result := toolsettest.CallTool(t, session, "paint", map[string]any{"color": "red"})

	if !result.IsError || toolsettest.ResultText(t, result) != "genwork API answered 409 label_name_taken" {
		t.Fatalf("result = %+v", result)
	}
}

func TestForwardAnswersTheConfirmationWhenTheAPIAnswersNoBody(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	api.RespondEmpty(http.MethodDelete, "/api/things/1", http.StatusNoContent)
	requester := platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), apitest.NewStaticTokenSource("t"))

	output, err := toolset.Forward(context.Background(), requester, platform.APIRequest{Method: http.MethodDelete, Path: "/api/things/1"}, "thing deleted")

	if err != nil || output != toolset.Confirm("thing deleted") {
		t.Fatalf("output = %v, err = %v", output, err)
	}
}

func TestForwardPassesAPIErrorsThrough(t *testing.T) {
	api := apitest.NewFakeAPI(t)
	requester := platform.NewAuthorizedAPIClient(platform.NewAPIClient(api.URL(), http.DefaultClient), apitest.NewStaticTokenSource("t"))

	_, err := toolset.Forward(context.Background(), requester, platform.APIRequest{Method: http.MethodGet, Path: "/api/missing"}, "")

	var apiError *platform.APIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusNotFound {
		t.Fatalf("err = %v", err)
	}
}
