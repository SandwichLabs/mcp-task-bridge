package server

import (
	"bytes"
	"context"
	"fmt"
	"os"
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

func Run(taskfilePath string, serverName string) {
	config, err := inspector.Inspect(taskfilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error inspecting Taskfile: %v\n", err)
		return
	}

	hooks := &server.Hooks{}

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		fmt.Fprintf(os.Stderr, "beforeAny: %s, %v, %v\n", method, id, message)
	})
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		fmt.Fprintf(os.Stderr, "onSuccess: %s, %v, %v, %v\n", method, id, message, result)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		fmt.Fprintf(os.Stderr, "onError: %s, %v, %v, %v\n", method, id, message, err)
	})
	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest) {
		fmt.Fprintf(os.Stderr, "beforeInitialize: %v, %v\n", id, message)
	})
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, message any) error {
		fmt.Fprintf(os.Stderr, "AddOnRequestInitialization: %v, %v\n", id, message)
		// authorization verification and other preprocessing tasks are performed.
		return nil
	})
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		fmt.Fprintf(os.Stderr, "afterInitialize: %v, %v, %v\n", id, message, result)
	})
	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		fmt.Fprintf(os.Stderr, "afterCallTool: %v, %v, %v\n", id, message, result)
	})
	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		fmt.Fprintf(os.Stderr, "beforeCallTool: %v, %v\n", id, message)
	})

	tools := TranslateTtmcpTools(config)
	handler := createTaskHandler(taskfilePath)

	s := server.NewMCPServer(serverName, "1.0.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
		server.WithHooks(hooks),
	)
	for _, tool := range tools {
		s.AddTool(*tool, handler) // Dereference tool
	}

	err = server.ServeStdio(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serving MCP: %v\n", err)
	}
}
