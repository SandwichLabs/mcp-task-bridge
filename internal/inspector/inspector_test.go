package inspector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Helper function to create a mock Taskfile
func createMockTaskfile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	taskfilePath := filepath.Join(tmpDir, "Taskfile.yml")
	err := os.WriteFile(taskfilePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create mock Taskfile: %v", err)
	}
	return taskfilePath
}

func newMockCmdExecutor(t *testing.T, expectedCmdSubstring string, output string, errToReturn error) func(string, ...string) *exec.Cmd {
	t.Helper()
	return func(command string, args ...string) *exec.Cmd {
		cmdStr := command + " " + strings.Join(args, " ")
		if !strings.Contains(cmdStr, expectedCmdSubstring) {
			t.Logf("Warning: execCommand called with %s, but mock is for %s. Falling back to real exec.", cmdStr, expectedCmdSubstring)
			return exec.Command(command, args...)
		}

		cs := []string{"-test.run=TestHelperProcess", "--"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"STDOUT="+output,
		)
		if errToReturn != nil {
			cmd.Env = append(cmd.Env, "EXIT_CODE=1", "STDERR="+errToReturn.Error())
		} else {
			cmd.Env = append(cmd.Env, "EXIT_CODE=0")
		}
		return cmd
	}
}

// TestHelperProcess isn't a real test. It's used as a helper for setupMockCmd.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// GO_COMMAND might be useful if we need to check which command was intended
	// fmt.Fprintf(os.Stderr, "Helper process called for command: %s\n", os.Getenv("GO_COMMAND"))
	// fmt.Fprintf(os.Stderr, "Helper process STDOUT: %s\n", os.Getenv("STDOUT"))
	// fmt.Fprintf(os.Stderr, "Helper process STDERR: %s\n", os.Getenv("STDERR"))
	// fmt.Fprintf(os.Stderr, "Helper process EXIT_CODE: %s\n", os.Getenv("EXIT_CODE"))

	fmt.Fprint(os.Stdout, os.Getenv("STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("STDERR"))

	exitCode := 0
	if os.Getenv("EXIT_CODE") == "1" {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func TestDiscoverTasks(t *testing.T) {
	// This test will use the actual task binary if available,
	// or a mocked version if we implement more sophisticated mocking later.
	// For now, let's create a real Taskfile and use the actual task command.

	t.Run("successful discovery", func(t *testing.T) {
		taskfileContent := `
version: '3'
tasks:
  task1:
    desc: "This is task 1"
  task2:
    desc: "This is task 2"
`
		taskfilePath := createMockTaskfile(t, taskfileContent)
		mockExecutor := newMockCmdExecutor(t, "task --list --json", `{"tasks": [{"name": "task1", "desc": "This is task 1"}, {"name": "task2", "desc": "This is task 2"}]}`, nil)

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		expectedTasks := []string{"task1", "task2"}
		tasks, err := inspector.DiscoverTasks()

		if err != nil {
			t.Fatalf("DiscoverTasks() error = %v, wantErr %v", err, false)
		}
		if !reflect.DeepEqual(tasks, expectedTasks) {
			t.Errorf("DiscoverTasks() = %v, want %v", tasks, expectedTasks)
		}
	})

	t.Run("task command fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "") // Content doesn't matter for this case
		mockExecutor := newMockCmdExecutor(t, "task --list --json", "", fmt.Errorf("task command failed"))

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		_, err = inspector.DiscoverTasks()
		if err == nil {
			t.Fatalf("DiscoverTasks() error = nil, wantErr %v", true)
		}
	})

	t.Run("json unmarshalling fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")
		mockExecutor := newMockCmdExecutor(t, "task --list --json", `{"tasks": [{"name": "task1", "desc": "This is task 1"}`, nil) // Invalid JSON

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		_, err = inspector.DiscoverTasks()
		if err == nil {
			t.Fatalf("DiscoverTasks() error = nil, wantErr %v", true)
		}
		// Check if the error is due to JSON unmarshalling problem as logged by DiscoverTasks,
		// or if it's the direct unmarshalling error.
		// The function logs "Error unmarshalling JSON" and returns the original error.
		// So, we check if the original error is related to JSON syntax.
		if _, ok := err.(*json.SyntaxError); !ok && !strings.Contains(err.Error(), "unexpected end of JSON input") {
			// Add more checks if other json errors are expected
			t.Errorf("DiscoverTasks() error = %v (%T), want json.SyntaxError or similar", err, err)
		}
	})
}

func TestGetTaskDetails(t *testing.T) {
	t.Run("successful details parsing", func(t *testing.T) {
		taskfileContent := `
version: '3'
tasks:
  weather:
    desc: "Get the current weather forecast"
    summary: |
      Retrieve a weather forecast for the provided ZIPCODE.
      Usage: task weather ZIPCODE=<zip> ANOTHER_PARAM=value
`
		taskfilePath := createMockTaskfile(t, taskfileContent)
		mockSummaryOutput := `
task: weather
Retrieve a weather forecast for the provided ZIPCODE.
Usage: task weather ZIPCODE=<zip> ANOTHER_PARAM=value
Required:
  ZIPCODE: The zipcode to get the weather for.
`
		mockExecutor := newMockCmdExecutor(t, "task weather --summary", mockSummaryOutput, nil)

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		expectedDetails := &TaskDefinition{
			Name:        "weather",
			Description: "Retrieve a weather forecast for the provided ZIPCODE.",
			Usage:       "task weather ZIPCODE=<zip> ANOTHER_PARAM=value",
			Parameters: []TaskParameter{
				{Name: "ZIPCODE"},
				{Name: "ANOTHER_PARAM"},
			},
		}

		details, err := inspector.GetTaskDetails("weather")
		if err != nil {
			t.Fatalf("GetTaskDetails() error = %v, wantErr %v", err, false)
		}
		if details.Name != expectedDetails.Name {
			t.Errorf("GetTaskDetails() Name = %q, want %q", details.Name, expectedDetails.Name)
		}
		if strings.TrimSpace(details.Description) != strings.TrimSpace(expectedDetails.Description) {
			t.Errorf("GetTaskDetails() Description = %q, want %q", details.Description, expectedDetails.Description)
		}
		if details.Usage != expectedDetails.Usage {
			t.Errorf("GetTaskDetails() Usage = %q, want %q", details.Usage, expectedDetails.Usage)
		}
		if !reflect.DeepEqual(details.Parameters, expectedDetails.Parameters) {
			t.Errorf("GetTaskDetails() Parameters = %v, want %v", details.Parameters, expectedDetails.Parameters)
		}

	})

	t.Run("task summary command fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")
		mockExecutor := newMockCmdExecutor(t, "task test-task --summary", "", fmt.Errorf("summary command failed"))

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		_, err = inspector.GetTaskDetails("test-task")
		if err == nil {
			t.Fatalf("GetTaskDetails() error = nil, wantErr %v", true)
		}
	})

    t.Run("summary with no usage line", func(t *testing.T) {
		taskfileContent := `
version: '3'
tasks:
  simple:
    desc: "A simple task"
    summary: |
      This is just a simple task.
      It has no specific usage instructions here.
`
		taskfilePath := createMockTaskfile(t, taskfileContent)
		mockSummaryOutput := `
task: simple
This is just a simple task.
It has no specific usage instructions here.
`
		mockExecutor := newMockCmdExecutor(t, "task simple --summary", mockSummaryOutput, nil)

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		expectedDetails := &TaskDefinition{
			Name:        "simple",
			Description: "This is just a simple task.\nIt has no specific usage instructions here.",
			Usage:       "", // Expect empty usage
			Parameters:  []TaskParameter{},
		}

		details, err := inspector.GetTaskDetails("simple")
		if err != nil {
			t.Fatalf("GetTaskDetails() error = %v, wantErr %v", err, false)
		}
		if details.Name != expectedDetails.Name {
			t.Errorf("GetTaskDetails() Name = %q, want %q", details.Name, expectedDetails.Name)
		}
		if strings.TrimSpace(details.Description) != strings.TrimSpace(expectedDetails.Description) {
			t.Errorf("GetTaskDetails() Description = %q, want %q", details.Description, expectedDetails.Description)
		}
		if details.Usage != expectedDetails.Usage {
			t.Errorf("GetTaskDetails() Usage = %q, want %q", details.Usage, expectedDetails.Usage)
		}
		if len(details.Parameters) != 0 {
			t.Errorf("GetTaskDetails() Parameters = %v, want empty slice", details.Parameters)
		}
	})


	t.Run("summary with usage but no parameters", func(t *testing.T) {
		taskfileContent := `
version: '3'
tasks:
  usageonly:
    desc: "A task with usage but no params"
    summary: |
      This task has a usage line.
      Usage: task usageonly --flag
`
		taskfilePath := createMockTaskfile(t, taskfileContent)
		mockSummaryOutput := `
task: usageonly
This task has a usage line.
Usage: task usageonly --flag
`
		mockExecutor := newMockCmdExecutor(t, "task usageonly --summary", mockSummaryOutput, nil)

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		expectedDetails := &TaskDefinition{
			Name:        "usageonly",
			Description: "This task has a usage line.",
			Usage:       "task usageonly --flag",
			Parameters:  []TaskParameter{},
		}

		details, err := inspector.GetTaskDetails("usageonly")
		if err != nil {
			t.Fatalf("GetTaskDetails() error = %v, wantErr %v", err, false)
		}
		if details.Name != expectedDetails.Name {
			t.Errorf("GetTaskDetails() Name = %q, want %q", details.Name, expectedDetails.Name)
		}
		if strings.TrimSpace(details.Description) != strings.TrimSpace(expectedDetails.Description) {
			t.Errorf("GetTaskDetails() Description = %q, want %q", details.Description, expectedDetails.Description)
		}
		if details.Usage != expectedDetails.Usage {
			t.Errorf("GetTaskDetails() Usage = %q, want %q", details.Usage, expectedDetails.Usage)
		}
		if len(details.Parameters) != 0 {
			t.Errorf("GetTaskDetails() Parameters = %v, want empty slice", details.Parameters)
		}
	})
}

func TestInspect(t *testing.T) {
	t.Run("successful inspection", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "version: '3'")

		mockExecutor := func(command string, args ...string) *exec.Cmd {
			var output string
			switch {
			case strings.Contains(strings.Join(args, " "), "--list --json"):
				output = `{"tasks": [{"name": "task1"}, {"name": "task2"}]}`
			case strings.Contains(strings.Join(args, " "), "task1 --summary"):
				output = "task: task1\nDesc 1\nUsage: Usage 1"
			case strings.Contains(strings.Join(args, " "), "task2 --summary"):
				output = "task: task2\nDesc 2\nUsage: Usage 2"
			}
			cs := []string{"-test.run=TestHelperProcess", "--"}
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+output, "EXIT_CODE=0")
			return cmd
		}

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		expectedConfig := &MCPConfig{
			Tasks: []TaskDefinition{
				{Name: "task1", Description: "Desc 1", Usage: "Usage 1"},
				{Name: "task2", Description: "Desc 2", Usage: "Usage 2"},
			},
		}

		config, err := inspector.Inspect()
		if err != nil {
			t.Fatalf("Inspect() error = %v, wantErr %v", err, false)
		}
		if !reflect.DeepEqual(config, expectedConfig) {
			t.Errorf("Inspect() config = \n%+v, want \n%+v", config, expectedConfig)
		}
	})

	t.Run("DiscoverTasks fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")
		mockExecutor := newMockCmdExecutor(t, "task --list --json", "", fmt.Errorf("discover failed"))

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		_, err = inspector.Inspect()
		if err == nil {
			t.Fatalf("Inspect() error = nil, wantErr %v", true)
		}
	})

		t.Run("GetTaskDetails fails for one task", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")

		mockExecutor := func(command string, args ...string) *exec.Cmd {
			var output, stderr string
			exitCode := "0"
			switch {
			case strings.Contains(strings.Join(args, " "), "--list --json"):
				output = `{"tasks": [{"name": "task1"}, {"name": "task2"}]}`
			case strings.Contains(strings.Join(args, " "), "task1 --summary"):
				output = "task: task1\nDesc 1\nUsage: Usage 1"
			case strings.Contains(strings.Join(args, " "), "task2 --summary"):
				stderr = "get details failed"
				exitCode = "1"
			}
			cs := []string{"-test.run=TestHelperProcess", "--"}
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+output, "STDERR="+stderr, "EXIT_CODE="+exitCode)
			return cmd
		}

		inspector, err := New(WithTaskfile(taskfilePath), withCmdExecutor(mockExecutor))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		_, err = inspector.Inspect()
		if err == nil {
			t.Fatalf("Inspect() error = nil, wantErr %v", true)
		}
	})
}
