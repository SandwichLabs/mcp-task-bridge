package inspector

import (
	"bytes"
	"log/slog"
	"os/exec"
	"strings"
)

type TaskResult struct {
	Name        string `json:"name"`
	TaskKey     string `json:"task"`
	Description string `json:"desc"`
	Usage       string `json:"usage"`
	Summary     string `json:"summary"`
}

type TaskListResult struct {
	Tasks []TaskResult `json:"tasks"`
}

func DiscoverTasks(taskfilePath string) ([]string, error) {
	slog.Info("Running command", "cmd", "task --list --taskfile "+taskfilePath)
	cmd := exec.Command("task", "--list", "--taskfile", taskfilePath)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		slog.Error("Error running task command", "error", err)
		return nil, err
	}
	lines := strings.Split(out.String(), "\n")
	slog.Info("Command output", "output", out.String())
	var tasks []string
	for _, line := range lines {
		if strings.HasPrefix(line, "* ") {
			slog.Info("Found task", "line", line)
			taskName := strings.Split(line, ":")[0]
			tasks = append(tasks, strings.TrimSpace(strings.TrimPrefix(taskName, "* ")))
		}
	}
	return tasks, nil
}

func GetTaskDetails(taskfilePath, taskName string) (*TaskDefinition, error) {
	slog.Info("Running command to get task details", "cmd", "task "+taskName+" --summary --taskfile "+taskfilePath)
	cmd := exec.Command("task", taskName, "--summary", "--taskfile", taskfilePath)
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
		slog.Info("Processing line", "line", line)

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
	slog.Info("Parsed task details", "taskName", taskName, "description", details.Description, "usage", details.Usage)
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

func Inspect(taskfilePath string) (*MCPConfig, error) {
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
