package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func SetupTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "get_todos", Description: "returns my todos"}, GetTodos)
}

type Input struct{}
type Output struct {
	Todos string `json:"todos" jsonschema:"todos"`
}

func GetTodos(ctx context.Context, req *mcp.CallToolRequest, input Input) (*mcp.CallToolResult, Output, error) {
	return nil, Output{
		Todos: gettodos(),
	}, nil
}

func gettodos() string {
	path := os.Getenv("TODOS_PATH")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading todos, the tool failed :( %s: %v\n", path, err)
	}
	return string(content)
}
