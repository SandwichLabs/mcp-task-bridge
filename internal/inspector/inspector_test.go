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

// Store the original cmdExec function
var originalCmdExec = cmdExec

func setupMockCmd(t *testing.T, expectedCmdSubstring string, output string, errToReturn error) {
	t.Helper()
	cmdExec = func(command string, args ...string) *exec.Cmd {
		cmdStr := command + " " + strings.Join(args, " ")
		if !strings.Contains(cmdStr, expectedCmdSubstring) {
			t.Logf("Warning: execCommand called with %s, but mock is for %s. Falling back to original behavior for this call.", cmdStr, expectedCmdSubstring)
			// To prevent nil pointer dereference if an unexpected command is called,
			// and to allow some flexibility if other commands are called by the functions under test
			// that are not the primary one being mocked.
			return originalCmdExec(command, args...)
		}

		cs := []string{"-test.run=TestHelperProcess", "--"}
		// cs = append(cs, command) // command is already part of os.Args[0] in helper
		// cs = append(cs, args...) // args are also part of os.Args for helper
		cmd := originalCmdExec(os.Args[0], cs...) // Use originalCmdExec to avoid recursion if os.Args[0] is `task`
		cmd.Env = append(os.Environ(), // Keep existing env
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

func teardownMockCmd() {
	cmdExec = originalCmdExec
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
		setupMockCmd(t, "task --list --json", `{"tasks": [{"name": "task1", "desc": "This is task 1"}, {"name": "task2", "desc": "This is task 2"}]}`, nil)
		defer teardownMockCmd()

		expectedTasks := []string{"task1", "task2"}
		tasks, err := DiscoverTasks(taskfilePath)

		if err != nil {
			t.Fatalf("DiscoverTasks() error = %v, wantErr %v", err, false)
		}
		if !reflect.DeepEqual(tasks, expectedTasks) {
			t.Errorf("DiscoverTasks() = %v, want %v", tasks, expectedTasks)
		}
	})

	t.Run("task command fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "") // Content doesn't matter for this case
		setupMockCmd(t, "task --list --json", "", fmt.Errorf("task command failed"))
		defer teardownMockCmd()

		_, err := DiscoverTasks(taskfilePath)
		if err == nil {
			t.Fatalf("DiscoverTasks() error = nil, wantErr %v", true)
		}
	})

	t.Run("json unmarshalling fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")
		setupMockCmd(t, "task --list --json", `{"tasks": [{"name": "task1", "desc": "This is task 1"}`, nil) // Invalid JSON
		defer teardownMockCmd()

		_, err := DiscoverTasks(taskfilePath)
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
		setupMockCmd(t, "task weather --summary", mockSummaryOutput, nil)
		defer teardownMockCmd()

		expectedDetails := &TaskDefinition{
			Name:        "weather",
			Description: "Retrieve a weather forecast for the provided ZIPCODE.",
			Usage:       "task weather ZIPCODE=<zip> ANOTHER_PARAM=value",
			Parameters: []TaskParameter{
				{Name: "ZIPCODE"},
				{Name: "ANOTHER_PARAM"},
			},
		}

		details, err := GetTaskDetails(taskfilePath, "weather")
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
		setupMockCmd(t, "task test-task --summary", "", fmt.Errorf("summary command failed"))
		defer teardownMockCmd()

		_, err := GetTaskDetails(taskfilePath, "test-task")
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
		setupMockCmd(t, "task simple --summary", mockSummaryOutput, nil)
		defer teardownMockCmd()

		expectedDetails := &TaskDefinition{
			Name:        "simple",
			Description: "This is just a simple task.\nIt has no specific usage instructions here.",
			Usage:       "", // Expect empty usage
			Parameters:  []TaskParameter{},
		}

		details, err := GetTaskDetails(taskfilePath, "simple")
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
		setupMockCmd(t, "task usageonly --summary", mockSummaryOutput, nil)
		defer teardownMockCmd()

		expectedDetails := &TaskDefinition{
			Name:        "usageonly",
			Description: "This task has a usage line.",
			Usage:       "task usageonly --flag",
			Parameters:  []TaskParameter{},
		}

		details, err := GetTaskDetails(taskfilePath, "usageonly")
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

// TestInspect now relies on mocking the cmdExec function,
// which DiscoverTasks and GetTaskDetails use.
func TestInspect(t *testing.T) {
	t.Run("successful inspection", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "version: '3'") // Content doesn't really matter due to mocking

		// Mock for DiscoverTasks call
		setupMockCmd(t, "task --list --json", `{"tasks": [{"name": "task1", "desc": "Desc 1"}, {"name": "task2", "desc": "Desc 2"}]}`, nil)
		// We need to be careful here. setupMockCmd is global.
		// The first call to cmdExec will be for DiscoverTasks.
		// Subsequent calls will be for GetTaskDetails.
		// This simple mock setup will apply the *last* setupMockCmd for all calls.
		// This is a limitation of the current simple mocking strategy.
		// For a more robust test, we would need a more sophisticated mock that can handle sequential calls
		// or differentiate based on exact command arguments.

		// For this test, we'll mock GetTaskDetails specifically for task1 and task2
		// This means DiscoverTasks will use the mock for task2's summary, which is not ideal but will pass.
		// A better way would be to have setupMockCmd accept a map of expectedCmd -> output, or a sequence of mocks.

		// Mock for GetTaskDetails call for task1
		// Since setupMockCmd overwrites, we need to set up mocks in the order they are NOT called or use a more complex mock.
		// Let's try to make setupMockCmd a bit smarter or chain them carefully.

		// Store original cmdExec to restore it between setups
		originalCmdExecForInspect := cmdExec

		// Setup for DiscoverTasks
		cmdExec = func(command string, args ...string) *exec.Cmd {
			cmdStr := command + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "task --list --json") {
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+`{"tasks": [{"name": "task1"}, {"name": "task2"}]}`, "EXIT_CODE=0")
				return cmd
			}
			if strings.Contains(cmdStr, "task1 --summary") {
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+"task: task1\nDesc 1\nUsage: Usage 1", "EXIT_CODE=0")
				return cmd
			}
			if strings.Contains(cmdStr, "task2 --summary") {
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+"task: task2\nDesc 2\nUsage: Usage 2", "EXIT_CODE=0")
				return cmd
			}
			// Fallback for any other command
			return originalCmdExec(command, args...)
		}
		defer func() { cmdExec = originalCmdExecForInspect }()


		expectedConfig := &MCPConfig{
			Tasks: []TaskDefinition{
				{Name: "task1", Description: "Desc 1", Usage: "Usage 1"},
				{Name: "task2", Description: "Desc 2", Usage: "Usage 2"},
			},
		}

		config, err := Inspect(taskfilePath)
		if err != nil {
			t.Fatalf("Inspect() error = %v, wantErr %v", err, false)
		}
		if !reflect.DeepEqual(config, expectedConfig) {
			t.Errorf("Inspect() config = \n%+v, want \n%+v", config, expectedConfig)
		}
	})

	t.Run("DiscoverTasks fails", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")
		setupMockCmd(t, "task --list --json", "", fmt.Errorf("discover failed"))
		defer teardownMockCmd()

		_, err := Inspect(taskfilePath)
		if err == nil {
			t.Fatalf("Inspect() error = nil, wantErr %v", true)
		}
		// The error returned by Inspect when DiscoverTasks fails should be the error from DiscoverTasks,
		// which is ultimately from cmd.Run() in the mock (exit status 1).
		if err.Error() != "exit status 1" {
			t.Errorf("Inspect() error = %q, want %q", err.Error(), "exit status 1")
		}
	})

	t.Run("GetTaskDetails fails for one task", func(t *testing.T) {
		taskfilePath := createMockTaskfile(t, "")

		originalCmdExecForInspect := cmdExec
		// The "get details failed" is what we pipe to STDERR in the mock,
		// but the actual error from cmd.Run() when exit code is 1 will be "exit status 1".
		// The functions GetTaskDetails and Inspect will return this "exit status 1" error.
		expectedErrFromCmd := "exit status 1"


		cmdExec = func(command string, args ...string) *exec.Cmd {
			cmdStr := command + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "task --list --json") {
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+`{"tasks": [{"name": "task1"}, {"name": "task2"}]}`, "EXIT_CODE=0")
				return cmd
			}
			if strings.Contains(cmdStr, "task1 --summary") { // task1 succeeds
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+"task: task1\nDesc 1\nUsage: Usage 1", "EXIT_CODE=0")
				return cmd
			}
			if strings.Contains(cmdStr, "task2 --summary") { // task2 fails
				cs := []string{"-test.run=TestHelperProcess", "--"}
				cmd := originalCmdExec(os.Args[0], cs...)
				// We set STDERR to "get details failed" but EXIT_CODE=1 causes cmd.Run() to return "exit status 1"
				cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "STDOUT="+"", "STDERR="+"get details failed", "EXIT_CODE=1")
				return cmd
			}
			return originalCmdExec(command, args...)
		}
		defer func() { cmdExec = originalCmdExecForInspect }()


		_, err := Inspect(taskfilePath)
		if err == nil {
			t.Fatalf("Inspect() error = nil, wantErr %v", true)
		}
		// The error from GetTaskDetails (which is from cmd.Run()) is returned by Inspect.
		if err.Error() != expectedErrFromCmd {
		    t.Errorf("Inspect() error = %q, want %q", err.Error(), expectedErrFromCmd)
		}
	})
}
