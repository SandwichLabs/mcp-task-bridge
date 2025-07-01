package inspector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// cmdExec is a package-level variable that can be swapped out for testing.
var cmdExec = exec.Command

// InspectFunc is a function variable that can be swapped out for testing.

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

type TaskInspector struct {
	TaskfilePath string
	TaskBinPath  string
}

type TaskInspectorService interface {
	WithTaskfile(taskfilePath string) (*TaskInspector, error)
	WithTaskBin(taskBinPath string) (*TaskInspector, error)
	DiscoverTasks() ([]string, error)
	GetTaskDetails(taskName string) (*TaskDefinition, error)
	Inspect(taskfilePath string) (*MCPConfig, error)
}

type TaskInspectorOption func(*TaskInspector) (*TaskInspector, error)

func New() *TaskInspector {
	return &TaskInspector{
		TaskfilePath: "Taskfile.yml", // Default to "Taskfile.yml"
		TaskBinPath:  "task",         // Default to "task" binary
	}
}

func (ti *TaskInspector) WithTaskfile(taskfilePath string) (*TaskInspector, error) {
	// Validate the taskfile path
	if taskfilePath == "" {
		return nil, fmt.Errorf("taskfile path cannot be empty")
	}
	// Validate the taskfile exists
	if _, err := exec.LookPath(taskfilePath); err != nil {
		return nil, fmt.Errorf("taskfile not found: %s", taskfilePath)
	}

	slog.Debug("Setting taskfile path", "path", taskfilePath)

	ti.TaskfilePath = taskfilePath
	return ti, nil
}

func (ti *TaskInspector) WithTaskBin(taskBinPath string) (*TaskInspector, error) {

	ti.TaskBinPath = taskBinPath
	if ti.TaskBinPath == "" {
		ti.TaskBinPath = "task" // Default to "task" if no path is provided
	}
	// Validate the task binary exists
	if _, err := exec.LookPath(ti.TaskBinPath); err != nil {
		return nil, fmt.Errorf("task binary not found, please install it or ensure it is available: %s", ti.TaskBinPath)
	}

	return ti, nil
}

func (ti *TaskInspector) DiscoverTasks() ([]string, error) {
	slog.Debug("Discovering tasks in", "path", ti.TaskfilePath)
	cmd := cmdExec(ti.TaskBinPath, "--list", "--json", "--taskfile", ti.TaskfilePath)

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

func (ti *TaskInspector) GetTaskDetails(taskName string) (*TaskDefinition, error) {
	slog.Debug("Getting details for", "task", taskName)
	cmd := cmdExec(ti.TaskBinPath, taskName, "--summary", "--taskfile", ti.TaskfilePath)
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

func (ti *TaskInspector) Inspect(taskfilePath string) (*MCPConfig, error) {
	taskNames, err := ti.DiscoverTasks()
	if err != nil {
		return nil, err
	}

	config := &MCPConfig{}
	for _, taskName := range taskNames {
		details, err := ti.GetTaskDetails(taskName)
		if err != nil {
			return nil, err
		}
		config.Tasks = append(config.Tasks, *details)
	}

	return config, nil
}
