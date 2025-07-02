package inspector

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os/exec"
	"strings"
)

// cmdExec is a package-level variable that can be swapped out for testing.
var cmdExec = exec.Command
var taskBin = "task"

// InspectFunc is a function variable that can be swapped out for testing.
var InspectFunc = Inspect

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

func DiscoverTasks(taskfilePath string) ([]string, error) {
	slog.Debug("Discovering tasks in", "path", taskfilePath)
	cmd := cmdExec(taskBin, "--list", "--json", "--taskfile", taskfilePath)

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

func GetTaskDetails(taskfilePath, taskName string) (*TaskDefinition, error) {
	slog.Debug("Getting details for", "task", taskName)
	cmd := cmdExec(taskBin, taskName, "--summary", "--taskfile", taskfilePath)
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

func Inspect(taskBinPath string, taskfilePath string) (*MCPConfig, error) {
	if taskBinPath != "" {
		taskBin = taskBinPath
	}

	taskNames, err := DiscoverTasks(taskfilePath)
	if err != nil {
		return nil, err
	}

	config := &MCPConfig{}
	for _, taskName := range taskNames {
		details, err := GetTaskDetails(taskfilePath, taskName)
		if err != nil {
			return nil, err
		}
		config.Tasks = append(config.Tasks, *details)
	}

	return config, nil
}
