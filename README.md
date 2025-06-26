# OMCP (Orndorff MCP) Task Bridge

`omcp` is a powerful Go CLI tool designed to bridge your existing `Taskfile.yml` defined shell commands with the Model Context Protocol (MCP). It allows you to expose your project's tasks as discoverable and executable tools for AI agents, leveraging `task --summary` for rich documentation.

## Why OMCP?

- **AI Agent Integration:** Seamlessly integrate your existing shell scripts and commands, defined in `Taskfile.yml`, with AI agents that understand the Model Context Protocol.
- **Structured Tooling:** Provides a structured way for less capable LLMs to interact with shell functions, offering constraints and clear interfaces.
- **Developer-Friendly:** Utilize a simple, `Makefile`-like syntax (`Taskfile.yml`) to define and expose MCP servers.
- **Automated Documentation:** Automatically generates MCP tool documentation by parsing `task --summary` output, providing LLMs with detailed usage instructions.

## How it Works

When an AI agent decides to execute a tool, `omcp` translates the MCP tool configuration into the correct `task` command syntax and executes it.

**Example:**

If an AI asks for the `weather` with parameters: `{ZIPCODE: 60626}`, `omcp` executes:

```bash
task -t Taskfile weather ZIPCODE=60626
```

...and responds with the results.

Similarly, for a news query:

```bash
task news
```

`omcp` runs the preconfigured commands, such as:

```bash
odt net:fetch:page:convert -- --url https://lite.cnn.com/
```

...and returns the markdown version of the CNN homepage for the agent to process.

## Commands

### Default Command (MCP Server)

The primary mode of operation for `omcp` is to act as an MCP server, exposing your `Taskfile` tasks as tools.

**Usage:**

```bash
omcp "path/to/Taskfile.yml"
omcp "Taskfile.yml" # If in the current directory
```

When invoked, `omcp` internally inspects the specified `Taskfile.yml` and then starts an MCP server configured with the introspected tasks as MCP tools. This server communicates over STDIN/STDOUT.

### `inspect` Command

The `inspect` command allows you to preview the MCP configuration that `omcp` would generate from your `Taskfile.yml` without starting the server.

**Usage:**

```bash
omcp inspect Taskfile.yml
```

This command runs `task` in a forked process to:
1.  List all available tasks (`task --list-all`).
2.  For each task, retrieve its summary and parameter details (`task <task name> --summary`).

The output is a JSON representation of the MCP server configuration, similar to a Swagger/OpenAPI specification, detailing the available tools and their options.

### `view` Command

The `view` command provides an interactive Text User Interface (TUI) to explore the MCP configuration derived from your `Taskfile.yml`.

**Usage:**

```bash
omcp view Taskfile.yml
```

This command internally runs `inspect` and then displays the MCP configuration in a BubbleTea TUI. You can browse available tools, view their descriptions, and inspect their parameters in a user-friendly interface.

## Installation

To install `omcp`, ensure you have Go installed and configured.

```bash
go install github.com/zac/omcp@latest
```

Alternatively, you can clone the repository and build from source:

```bash
git clone https://github.com/zac/omcp.git
cd omcp
go build -o omcp .
```

## Development

`omcp` is built with Go and leverages the following key technologies:

-   **CLI Framework:** [Cobra](https://github.com/spf13/cobra) for robust command-line interface handling.
-   **TUI Framework:** [BubbleTea](https://github.com/charmbracelet/bubbletea) for the interactive `view` command.
-   **MCP Library:** `github.com/mark3labs/mcp-go` for Model Context Protocol integration.

Contributions are welcome! Please refer to `PLAN.md` for detailed implementation plans and `MCP_DEVELOPMENT.md` for specific development guidelines.
