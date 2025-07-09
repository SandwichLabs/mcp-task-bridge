# DEVELOPERS.md

This document provides a technical overview of the `tmcp` CLI project for developers.

## Project Overview

`tmcp` is a command-line tool that dynamically creates a Machine-to-Machine Communication Protocol (MCP) server or a Langchain Go Agent from a `Taskfile.yml`. It works by introspecting a `Taskfile` to find available tasks and their descriptions, then exposing them as callable tools for other services or AI agents.

The primary goals are:
- To provide a simple, `make`-like interface (`Taskfile.yml`) for defining shell-based tools.
- To expose these tools over a standardized protocol (MCP) for machine interaction.
- To enable AI agents (using Langchain Go) to use these shell-based tools to perform actions.

## Architecture and Project Structure

The project is written in Go and uses the [Cobra](https://github.com/spf13/cobra) library for its CLI structure. The logic is separated into `cmd` for command-line interfacing and `internal` for the core business logic.

```
/
├── cmd/                    # Cobra command definitions
│   ├── root.go             # Default command (runs MCP server)
│   ├── agent.go            # 'agent' command (runs Langchain agent)
│   ├── inspect.go          # 'inspect' command (outputs JSON config)
│   └── view.go             # 'view' command (interactive TUI)
│
├── internal/               # Core business logic
│   ├── inspector/          # Logic for parsing Taskfiles
│   │   ├── inspector.go
│   │   └── types.go
│   ├── server/             # MCP server implementation
│   │   └── server.go
│   └── tui/                # BubbleTea TUI view
│       └── viewer.go
│
├── go.mod                  # Go modules
└── main.go                 # Main application entry point
```

### `main.go`
The entry point is minimal, simply calling `cmd.Execute()` to run the Cobra root command.

### `cmd/` Package
This package defines the CLI commands:
- **`root.go`**: Implements the default command (`tmcp [Taskfile]`). It inspects the Taskfile and starts the MCP server using the `internal/server` package.
- **`agent.go`**: Implements the `agent` command. It inspects the Taskfile, creates a set of `langchaingo/tools.Tool` implementations, and runs a Langchain agent that can use these tools.
- **`inspect.go`**: Implements the `inspect` command. It uses the `internal/inspector` to parse a Taskfile and prints the resulting MCP configuration as a JSON object to stdout.
- **`view.go`**: Implements the `view` command. It inspects the Taskfile and then uses the `internal/tui` package to display the configuration in an interactive terminal UI.

### `internal/` Package
This package contains the application's core logic:
- **`internal/inspector`**: This is the heart of the tool. It is responsible for shelling out to the `task` binary to understand the `Taskfile`.
  - `DiscoverTasks`: Runs `task --list --json` to get a list of all available task names.
  - `GetTaskDetails`: For each task, it runs `task <task_name> --summary` to parse its description and usage instructions.
- **`internal/server`**: This package sets up and runs the MCP server.
  - `TranslateTtmcpTools`: Converts the `TaskDefinition` structs from the inspector into `mcp.Tool` objects.
  - `createTaskHandler`: Creates a generic `server.ToolHandlerFunc` that executes the appropriate `task` command when an MCP tool is called.
- **`internal/tui`**: Implements the interactive view using the [BubbleTea](https://github.com/charmbracelet/bubbletea) framework.

## Core Mechanisms

### Task Inspection
The `internal/inspector` package is the key component. It does **not** parse the YAML of the `Taskfile` directly. Instead, it uses the `task` command-line tool itself as the source of truth.

1.  **Discovery**: `DiscoverTasks` calls `task --list --json --taskfile [path]`. It parses the JSON output to get the names of all tasks.
2.  **Detail Extraction**: For each task name, `GetTaskDetails` calls `task [task_name] --summary --taskfile [path]`. It then parses the human-readable text output to extract the task's description and usage string. This parsing is sensitive to the format of `task --summary`'s output.

### Agent Tool Implementation
In `cmd/agent.go`, the `taskExecutorTool` struct is the bridge to the Langchain framework.
- It implements the `tools.Tool` interface.
- The `Name()` method returns the task name.
- The `Description()` method returns a combination of the task's description and its usage string, which is crucial for the LLM to understand how to use the tool and what parameters it accepts.
- The `Call()` method is where the magic happens. It receives the agent's input, constructs the final `task ...` command with arguments, executes it, and returns the `stdout` and `stderr` to the agent.

## Testing Strategy

The project relies on shelling out to the `task` binary, which presents a challenge for testing. The solution implemented in `internal/inspector/inspector_test.go` is a common Go pattern for mocking external commands.

- **`TestHelperProcess`**: This special test function is not a real test. It's designed to be run as a subprocess by other tests.
- **Mocking `exec.Command`**: The `cmdExec` package-level variable holds the function used to create commands (defaults to `exec.Command`). In tests, this variable is replaced with a mock function.
- **The Mock Function**: The mock `cmdExec` function returns an `*exec.Cmd` that calls the test binary itself with the `-test.run=TestHelperProcess` flag.
- **Environment Variables**: The mock function uses environment variables (`STDOUT`, `STDERR`, `EXIT_CODE`) to tell the `TestHelperProcess` what to write to its stdout/stderr and what exit code to use.

This pattern allows tests to simulate different outputs and error conditions from the `task` binary without needing it to be installed and without the flakiness of filesystem interactions.

## How to Contribute and Extend

### Adding a New Command
1.  Create a new file in the `cmd/` directory (e.g., `mycommand.go`).
2.  Define a new `cobra.Command` variable.
3.  In the `init()` function of your new file, add flags if needed and register the command with `rootCmd.AddCommand(myCmd)`.

### Updating the Inspector
If the output format of `task --list --json` or `task --summary` changes in a future version of `go-task`, the parsing logic in `internal/inspector/inspector.go` will need to be updated. The tests in `internal/inspector/inspector_test.go` should be updated first to reflect the new output, which will then guide the required changes in the implementation.

### Supporting a New LLM Provider for the Agent
1.  Add a new case to the `switch provider` statement in `runAgent` (`cmd/agent.go`).
2.  Add a function to get the API key (e.g., `getNewProviderToken()`).
3.  To support testing, add a new function variable for the provider's constructor (e.g., `var newMyLLMFn = myllm.New`) and mock it in `cmd/agent_skip.go`.

## How to build

To build the `tmcp` binary from source, use the following command:

```bash
go build -o tmcp .
```

This will create a `tmcp` executable in the project's root directory.

## How to run tests

The project includes a suite of tests to ensure its functionality. To run the tests, use the following command:

```bash
go test ./...
```

## System Dependencies

The `tmcp` tool has one main system dependency:

- **Go**: The Go programming language is required to build and run the project. You can find installation instructions at [https://go.dev/doc/install](https://go.dev/doc/install).
- **task**: The `task` binary must be installed and available in the system's `PATH`. You can find installation instructions at [https://taskfile.dev/installation/](https://taskfile.dev/installation/).
