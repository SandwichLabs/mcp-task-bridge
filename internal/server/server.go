package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
)

func TranslateTtmcpTools(config *inspector.MCPConfig) []*mcp.Tool {
	var tools []*mcp.Tool
	for _, task := range config.Tasks {
		var toolOptions []mcp.ToolOption
		toolOptions = append(toolOptions, mcp.WithDescription(task.Description))
		for _, param := range task.Parameters {
			toolOptions = append(toolOptions, mcp.WithString(param.Name, mcp.Required()))
		}
		tool := mcp.NewTool(task.Name, toolOptions...)
		tools = append(tools, &tool) // Take address of tool
	}
	return tools
}

func createTaskHandler(taskfilePath string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args []string
		args = append(args, "--taskfile", taskfilePath, request.Params.Name)
		for key, value := range request.GetArguments() {
			args = append(args, fmt.Sprintf("%s=%s", key, value))
		}

		cmd := exec.Command("task", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			return mcp.NewToolResultError(stderr.String()), nil
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

func Run(taskfilePath string) {
	config, err := inspector.Inspect(taskfilePath)
	if err != nil {
		log.Fatalf("Error inspecting Taskfile: %v", err)
	}

	tools := TranslateTtmcpTools(config)
	handler := createTaskHandler(taskfilePath)

	s := server.NewMCPServer("tasks", "1.0.0")
	for _, tool := range tools {
		s.AddTool(*tool, handler) // Dereference tool
	}

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
	}
}
