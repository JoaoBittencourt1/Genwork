package toolset

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool is one MCP tool and the permission a caller needs to see it; an empty Permission means any signed-in user.
type Tool struct {
	Definition *mcp.Tool
	Permission string
	addTo      func(server *mcp.Server)
}

type ToolCall[In any] func(ctx context.Context, input In) (any, error)

// NewTool infers the input schema from In; enums restrict string properties to the values the API accepts.
func NewTool[In any](definition *mcp.Tool, permission string, enums map[string][]any, call ToolCall[In]) Tool {
	definition.InputSchema = inputSchemaFor[In](definition.Name, enums)
	return Tool{
		Definition: definition,
		Permission: permission,
		addTo: func(server *mcp.Server) {
			mcp.AddTool(server, definition, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
				output, err := call(ctx, input)
				return nil, output, err
			})
		},
	}
}

// Confirmation is the output of tools whose API call answers no body.
type Confirmation struct {
	Result string `json:"result"`
}

func Confirm(result string) Confirmation {
	return Confirmation{Result: result}
}

func ReadOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(false)}
}

func Writes() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)}
}

func Destroys() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)}
}

func inputSchemaFor[In any](toolName string, enums map[string][]any) *jsonschema.Schema {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Sprintf("tool %s: infer input schema: %v", toolName, err))
	}
	for property, values := range enums {
		propertySchema, ok := schema.Properties[property]
		if !ok {
			panic(fmt.Sprintf("tool %s: enum for unknown property %q", toolName, property))
		}
		propertySchema.Enum = values
	}
	return schema
}
