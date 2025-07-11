# tmcp Task to Mcp Bridge

Extremely early development 

Don't use this for anything in production, I'm just noodling with MCP here.



`tmcp` is a powerful Go CLI tool designed to bridge your existing `Taskfile.yml` defined shell commands with the Model Context Protocol (MCP). It allows you to expose your project's tasks as discoverable and executable tools for AI agents, leveraging `task --summary` for rich documentation.

## Why tmcp?

- **AI Agent Integration:** Seamlessly integrate your existing shell scripts and commands, defined in `Taskfile.yml`, with AI agents that understand the Model Context Protocol.
- **Structured Tooling:** Provides a structured way for less capable LLMs to interact with shell functions, offering constraints and clear interfaces.
- **Developer-Friendly:** Utilize a simple, `Makefile`-like syntax (`Taskfile.yml`) to define and expose MCP servers.
- **Automated Documentation:** Automatically generates MCP tool documentation by parsing `task --summary` output, providing LLMs with detailed usage instructions.

## How it Works

`tmcp` works by inspecting a `Taskfile.yml` to discover the available tasks and their configurations. This inspection process is done by shelling out to the `task` binary itself, ensuring that `tmcp` respects the full capabilities of `task`.

Here's a step-by-step breakdown of how `tmcp` creates an MCP server:

1.  **Task Discovery**: `tmcp` first runs `task --list-all` to get a list of all the available tasks in the `Taskfile.yml`.

2.  **Detail Extraction**: For each task discovered, `tmcp` runs `task <task_name> --summary` to extract the task's description, usage instructions, and any parameters it requires.

3.  **MCP Tool Generation**: The information gathered in the previous steps is used to generate a corresponding `mcp.Tool` for each task. The task's name becomes the tool's name, the description becomes the tool's description, and the usage instructions are used to define the tool's parameters.

4.  **MCP Server Creation**: Finally, `tmcp` creates an MCP server and registers all the generated tools with it. This server listens for incoming requests from AI agents.

When an AI agent decides to execute a tool, `tmcp` translates the MCP tool configuration into the correct `task` command syntax and executes it.

**Example:**

If an AI asks for the `weather` with parameters: `{ZIPCODE: 60626}`, `tmcp` executes:

```bash
task -t Taskfile weather ZIPCODE=60626
```

...and responds with the results.

Similarly, for a news query:

```bash
task news
```

`tmcp` runs the preconfigured commands, such as:

```bash
odt net:fetch:page:convert -- --url https://lite.cnn.com/
```

...and returns the markdown version of the CNN homepage for the agent to process.

## Commands

### Default Command (MCP Server)

The primary mode of operation for `tmcp` is to act as an MCP server, exposing your `Taskfile` tasks as tools.

**Usage:**

```bash
tmcp "path/to/Taskfile.yml"
tmcp "Taskfile.yml" # If in the current directory
```

When invoked, `tmcp` internally inspects the specified `Taskfile.yml` and then starts an MCP server configured with the introspected tasks as MCP tools. This server communicates over STDIN/STDOUT.

### `inspect` Command

The `inspect` command allows you to preview the MCP configuration that `tmcp` would generate from your `Taskfile.yml` without starting the server.

**Usage:**

```bash
tmcp inspect Taskfile.yml
```

This command runs `task` in a forked process to:
1.  List all available tasks (`task --list-all`).
2.  For each task, retrieve its summary and parameter details (`task <task name> --summary`).

The output is a JSON representation of the MCP server configuration, similar to a Swagger/OpenAPI specification, detailing the available tools and their options.

### `view` Command

The `view` command provides an interactive Text User Interface (TUI) to explore the MCP configuration derived from your `Taskfile.yml`.

**Usage:**

```bash
tmcp view Taskfile.yml
```

This command internally runs `inspect` and then displays the MCP configuration in a BubbleTea TUI. You can browse available tools, view their descriptions, and inspect their parameters in a user-friendly interface.

## Installation

To install `tmcp`, download the latest release from the [GitHub Releases page](https://github.com/SandwichLabs/mcp-task-bridge/releases) or use the following command to install it via Go:

```bash
# Extract ze files
tar -xzf tmcp_darwin_arm64.tar.gz
# Move the binary to a directory in your PATH
mv tmcp /usr/local/bin/

```

To install `tmcp`, ensure you have Go installed and configured.

```bash
go install github.com/sandwichlabs/mcp-task-bridge@latest
```

Alternatively, you can clone the repository and build from source:

```bash
git clone https://github.com/sandwichlabs/mcp-task-bridge.git
cd tmcp
go build -o tmcp .
```


## Usage with Claude Code

`claude mcp add my_tasks tmcp "path/to/Taskfile.yml"`

## Usage sith Claude Desktop

Edit the `claude-desktop` config file to add the `tmcp` command:

```json
// /Users/{you}/Library/Application Support/Claude/claude_desktop_config.json

```
mcpServers: {
  my_tasks: {
    "name": "tmcp",
    "description": "Run tasks defined in Taskfile.yml",
    "command": "tmcp",
    "args": ["/absolute/path/to/Taskfile.yml"]
  }
}
```


Then, you can invoke tasks in Claude Desktop as:
`Check 'my task name' for X.`

## Development

`tmcp` is built with Go and leverages the following key technologies:

-   **CLI Framework:** [Cobra](https://github.com/spf13/cobra) for robust command-line interface handling.
-   **TUI Framework:** [BubbleTea](https://github.com/charmbracelet/bubbletea) for the interactive `view` command.
-   **MCP Library:** `github.com/mark3labs/mcp-go` for Model Context Protocol integration.
