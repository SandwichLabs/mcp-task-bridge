package inspector

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
)

// Inspector is responsible for inspecting a Taskfile.
type Inspector struct {
	taskBinPath  string
	taskfilePath string
	// For improved testability, we can also include the command executor here.
	cmdExecutor func(command string, args ...string) *exec.Cmd
}

// Option is a function that configures an Inspector.
type Option func(*Inspector)

// New creates a new Inspector with the given options.
func New(opts ...Option) (*Inspector, error) {
	// Start with default values
	inspector := &Inspector{
		taskBinPath: "task",
		cmdExecutor: exec.Command, // Default to the real exec.Command
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(inspector)
	}

	// Validate that required options were provided
	if inspector.taskfilePath == "" {
		return nil, errors.New("taskfile path is required")
	}

	return inspector, nil
}

// WithTaskfile sets the path to the Taskfile.
func WithTaskfile(path string) Option {
	return func(i *Inspector) {
		i.taskfilePath = path
	}
}

// WithTaskBin sets the path to the task binary.
func WithTaskBin(path string) Option {
	return func(i *Inspector) {
		i.taskBinPath = path
	}
}

// (For Testing) withCmdExecutor sets a custom command executor.
func withCmdExecutor(execFunc func(string, ...string) *exec.Cmd) Option {
	return func(i *Inspector) {
		i.cmdExecutor = execFunc
	}
}



type TaskResult struct {
	Name        string `json:"name"`
	TaskKey     string `json:"task"`
	Description string `json:"desc"`
	Usage       string `json:"usage"` // This field is not directly available in --list --json, summary contains it.
	Summary     string `json:"summary"`
}

type TaskListResult struct {
	Tasks []TaskResult `json:"tasks"`
}

// Inspect runs the full inspection process.
func (i *Inspector) Inspect() (*MCPConfig, error) {
	taskNames, err := i.DiscoverTasks()
	if err != nil {
		return nil, err
	}

	config := &MCPConfig{}
	for _, taskName := range taskNames {
		details, err := i.GetTaskDetails(taskName)
		if err != nil {
			return nil, err
		}
		config.Tasks = append(config.Tasks, *details)
	}

	return config, nil
}

// DiscoverTasks discovers the tasks in the configured Taskfile.
func (i *Inspector) DiscoverTasks() ([]string, error) {
	slog.Debug("Discovering tasks in", "path", i.taskfilePath)
	cmd := i.cmdExecutor(i.taskBinPath, "--list", "--json", "--taskfile", i.taskfilePath)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		slog.Error("Error running task command", "error", err, "output", out.String())
		return nil, err
	}

	slog.Debug("Marshalling json output")
	var taskListResult TaskListResult
	if err := json.Unmarshal(out.Bytes(), &taskListResult); err != nil {
		slog.Error("Error unmarshalling JSON from task list", "error", err)
		return nil, err
	}

	var tasks []string
	for _, task := range taskListResult.Tasks {
		tasks = append(tasks, task.Name)
	}
	slog.Debug("Discovered tasks", "task_count", len(tasks))
	return tasks, nil
}

// GetTaskDetails gets the details for a specific task.
func (i *Inspector) GetTaskDetails(taskName string) (*TaskDefinition, error) {
	slog.Debug("Getting details for", "task", taskName)
	cmd := i.cmdExecutor(i.taskBinPath, taskName, "--summary", "--taskfile", i.taskfilePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	details := &TaskDefinition{Name: taskName}
	parsingState := ""

	for _, line := range lines {
		slog.Debug("Processing line", "line", line)

		if strings.HasPrefix(line, "task: ") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "Usage:"):
			parsingState = "usage"
			details.Usage = strings.TrimSpace(strings.TrimPrefix(line, "Usage:"))
		case strings.HasPrefix(line, "Required:"):
			parsingState = "required"
			// Further parsing for required params can be done here
		default:
			if parsingState == "" {
				details.Description += line + "\n"
			}
		}
	}

	details.Description = strings.TrimSpace(details.Description)
	slog.Debug("Parsed task details", "taskName", taskName, "description", details.Description, "usage", details.Usage)
	// Basic parameter parsing from Usage line
	if strings.Contains(details.Usage, "=") {
		parts := strings.Split(details.Usage, " ")
		for _, part := range parts {
			if strings.Contains(part, "=") {
				paramName := strings.Split(part, "=")[0]
				details.Parameters = append(details.Parameters, TaskParameter{Name: paramName})
			}
		}
	}

	return details, nil
}
