# OMCP (Orndorff MCP) Tool Project

A cli built to bridge your existing shell tools with MCP.
`omcp ./Taskfile.yml` will evaluate the Taskfile tasks and expose each as an MCP function leveraging `task --summary` to provide llms with documentation on how to use each.

This is meant to provide contraints on less capable llms for use with Shell functions and for developers to utilize a simple makefile like syntax to define and expose mcp servers to their AI Agents.


When the `Agent` decides to execute a `tool`. The `tool` configuration will be translated to the correct `task` command syntax and executed.

Example.
AI asks for the `weather` with params: `{ZIPCODE: 60626}`. omcp executes `task -t Taskfile weather ZIPCODE=60626` and responds with the results.

## Commands

### Default Command

Usage:

- `omcp "path/to/TaskFile.yml"`
- `omcp "TaskFile.uml"`

When the default command is invoked we run `inspect`(internally) and then start an mcp server configured with the introspected tasks as mcp tools:

### Inspect

Usage:
`omcp inspect Taskfile.yml`

The inspect command will run `task {command flags}` in a forked process which will receive the results in stdout/stderr.

1. Run Task in current (live) directory(execute the `task` utility in a forked process), read the stdout to capture the list of available tasks. 

Example stdout from `task --list -t {Taskfile path passed to inspect command}`:

```
task: Available tasks for this project:
* news:          This task will get the latest news update
* weather:       Get the current weather forecast
```


2. For each available Task, run `task {task name} --summary`: to get a description of the task and options.

Example output from `task --summary -t {Taskfile path passed to inspect command}`

```
task: weather

Retrieve a weather forecast for the provided ZIPCODE.
Usage: task weather ZIPCODE=<zipcode here>
Required: ZIPCODE

```



Output when executed as a standalone command: JSON representation of the MCP server configuration, like a Swagger/Openapi spec showing what the tools and options are.

Internally it should exposed a method to retrieve it as a function response MCPConfig object.



### View

Usage:
- `omcp view Taskfile`

The `view` command  runs `inspect` (internally) and then displays the MCP configuration in an interactive BubbleTea TUI to allow for viewing the configured mcp server configuration. What tools are available, parameters, etc.



## Examples

Run parameterized queries against a pre-defined database with `dt`.

`Find me users in the system named bob.`

Agent translates that to the `query_users` task.

`task data:query_users SEARCH='bob'`

`Task` then executes the  preconfigured `dt` command.

`dt q "select * from users where name iLike='%?%'" -p $SEARCH`

`What's going on in the news today?`

Agent translates that to the `news` task.

`task news`

`task` is then executed with that task_name and runs the preconfigured commands.

`odt net:fetch:page:convert -- --url https://lite.cnn.com/`
Which returns the markdown version of the cnn homepage.
Agent then answers the question.

