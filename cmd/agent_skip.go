package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
)

// Stores captured options for newOpenAIFn or newAnthropicFn
var capturedOpenAIOptions []openai.Option
var capturedAnthropicOptions []anthropic.Option

// Mock a successful inspection
func mockInspect(taskBin string, taskfilePath string) (*inspector.MCPConfig, error) {
	return &inspector.MCPConfig{
		Tasks: []inspector.TaskDefinition{
			{Name: "test-task", Description: "A test task", Usage: "task test-task PARAM=value"},
			{Name: "another-task", Description: "Another test task", Usage: "task another-task"},
		},
	}, nil
}

func mockInspectError(_ string, _ string) (*inspector.MCPConfig, error) {
	return nil, assert.AnError
}

// Variables to store original functions that will be mocked
var (
	originalNewOpenAIFn    func(...openai.Option) (*openai.LLM, error)
	originalNewAnthropicFn func(...anthropic.Option) (*anthropic.LLM, error)
	originalInspectFunc    func(string, string) (*inspector.MCPConfig, error) // Changed name
)

func setupAgentTest() {
	// Store original functions before replacing them with mocks
	originalInspectFunc = inspector.InspectFunc // Use the new InspectFunc variable
	originalNewOpenAIFn = newOpenAIFn
	originalNewAnthropicFn = newAnthropicFn

	// Reset captured options for each test
	capturedOpenAIOptions = nil
	capturedAnthropicOptions = nil

	// Assign mocks
	inspector.InspectFunc = mockInspect // Mock the new InspectFunc variable

	newOpenAIFn = func(opts ...openai.Option) (*openai.LLM, error) {
		capturedOpenAIOptions = opts
		// Return a minimal, non-nil *openai.LLM.
		return &openai.LLM{}, nil
	}
	newAnthropicFn = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
		capturedAnthropicOptions = opts
		return &anthropic.LLM{}, nil
	}

	// Reset flags to their default values as defined in agent.go's init()
	provider = "anthropic"
	modelName = "claude-3-sonnet-20240229"
	temperature = 0.7
	maxTokens = 256
}

func teardownAgentTest(t *testing.T) {
	// Restore original functions
	inspector.InspectFunc = originalInspectFunc // Restore InspectFunc
	newOpenAIFn = originalNewOpenAIFn
	newAnthropicFn = originalNewAnthropicFn
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf) // Capture stdout
	root.SetErr(buf) // Capture stderr
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestAgentCommand_RunSuccess_AnthropicDefault(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--temperature", "0.5", "--max-tokens", "150")
	assert.NoError(t, err)

	assert.Contains(t, output, "Provider: anthropic")
	assert.Contains(t, output, "Model Name (configured in LLM client): claude-3-sonnet-20240229")
	assert.Contains(t, output, "Temperature: 0.500000")
	assert.Contains(t, output, "Max Tokens: 150")
	assert.Contains(t, output, "Name: test-task")
	assert.Contains(t, output, "Description & Usage: A test task Usage: task test-task PARAM=value")

	assert.NotNil(t, capturedAnthropicOptions, "newAnthropicFn should have been called with options")
	assert.Nil(t, capturedOpenAIOptions, "newOpenAIFn should NOT have been called")
}

func TestAgentCommand_RunSuccess_OpenAI(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--provider", "openai", "--model-name", "gpt-4-test", "--temperature", "0.2", "--max-tokens", "50")
	assert.NoError(t, err)

	assert.Contains(t, output, "Provider: openai")
	assert.Contains(t, output, "Model Name (configured in LLM client): gpt-4-test")
	assert.Contains(t, output, "Temperature: 0.200000")
	assert.Contains(t, output, "Max Tokens: 50")

	assert.NotNil(t, capturedOpenAIOptions, "newOpenAIFn should have been called with options")
	assert.Nil(t, capturedAnthropicOptions, "newAnthropicFn should NOT have been called")
}

func TestAgentCommand_InspectError(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)
	inspector.InspectFunc = mockInspectError // Mock InspectFunc to return an error
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml")
	assert.Contains(t, output, "Failed to inspect Taskfile")
}

func TestAgentCommand_OpenAIInitError(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)
	newOpenAIFn = func(opts ...openai.Option) (*openai.LLM, error) { // Mock newOpenAIFn to return an error
		return nil, assert.AnError
	}
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "openai")
	assert.Contains(t, output, "Failed to initialize OpenAI LLM")
}

func TestAgentCommand_AnthropicInitError(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)
	newAnthropicFn = func(opts ...anthropic.Option) (*anthropic.LLM, error) { // Mock newAnthropicFn
		return nil, assert.AnError
	}
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "anthropic")
	assert.Contains(t, output, "Failed to initialize Anthropic LLM")
}

func TestAgentCommand_UnsupportedProvider(t *testing.T) {
	setupAgentTest()
	defer teardownAgentTest(t)
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "unknown-provider")
	assert.Contains(t, output, "Unsupported LLM provider")
}

func TestTaskExecutorTool_Call(t *testing.T) {
	tool := &taskExecutorTool{
		taskName:        "test-exec",
		taskDescription: "Test execution",
		taskUsage:       "test-exec INPUT=val",
		taskfilePath:    "TestTaskfile.ymlForTool.yml",
	}
	dummyTaskContent := "version: '3'\ntasks:\n  test-exec:\n    cmds:\n      - echo \"Output for $INPUT\"\n    vars:\n      INPUT: \"default\""
	err := os.WriteFile(tool.taskfilePath, []byte(dummyTaskContent), 0600)
	assert.NoError(t, err)
	defer os.Remove(tool.taskfilePath)

	output, err := tool.Call(context.Background(), "INPUT=world")
	assert.NoError(t, err)
	assert.Contains(t, output, "Output for world")
}

func TestGetOpenAIToken(t *testing.T) {
	originalVal := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "env-key-openai")
	assert.Equal(t, "env-key-openai", getOpenAIToken())
	os.Unsetenv("OPENAI_API_KEY")
	assert.Equal(t, "sk-your-api-key-placeholder", getOpenAIToken())
	if originalVal != "" {
		os.Setenv("OPENAI_API_KEY", originalVal)
	} // Restore original
}

func TestGetAnthropicToken(t *testing.T) {
	originalVal := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "env-key-anthropic")
	assert.Equal(t, "env-key-anthropic", getAnthropicToken())
	os.Unsetenv("ANTHROPIC_API_KEY")
	assert.Equal(t, "anthropic-api-key-placeholder", getAnthropicToken())
	if originalVal != "" {
		os.Setenv("ANTHROPIC_API_KEY", originalVal)
	} // Restore original
}

func TestAgentCmdFlags(t *testing.T) {
	setupAgentTest() // Sets up mocks for LLM constructors and inspector
	defer teardownAgentTest(t)

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testCases := []struct {
		name             string
		args             []string // Args for the 'agent' command, including flags
		expectedProvider string   // Expected value of the global 'provider' variable after parsing
		expectedModel    string   // Expected value of the global 'modelName' variable
		expectedTemp     float64  // Expected value of the global 'temperature' variable
		expectedTokens   int      // Expected value of the global 'maxTokens' variable
	}{
		{
			"Defaults (Anthropic from setup)",
			[]string{"agent", dummyTaskfile}, // No flags, should use defaults
			"anthropic", "claude-3-sonnet-20240229", 0.7, 256,
		},
		{
			"Custom Anthropic Model and Temp",
			[]string{"agent", dummyTaskfile, "--model-name", "claude-opus-test", "--temperature", "0.3", "--max-tokens", "50"},
			"anthropic", "claude-opus-test", 0.3, 50,
		},
		{
			"Specify OpenAI Provider and Model",
			[]string{"agent", dummyTaskfile, "--provider", "openai", "--model-name", "gpt-4-turbo", "--temperature", "0.9", "--max-tokens", "1024"},
			"openai", "gpt-4-turbo", 0.9, 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global flag variables to their initial state before Cobra parsing for this specific test case
			// These are the defaults defined by agentCmd.Flags() if not overridden by tc.args
			provider = "anthropic"
			modelName = "claude-3-sonnet-20240229"
			temperature = 0.7
			maxTokens = 256

			capturedOpenAIOptions = nil    // Reset captured options
			capturedAnthropicOptions = nil // Reset captured options

			testRootCmd := &cobra.Command{Use: "tmcp"}
			testRootCmd.AddCommand(agentCmd)
			output, err := executeCommand(testRootCmd, tc.args...)
			assert.NoError(t, err)

			// 1. Check that the global flag variables were correctly updated by Cobra
			assert.Equal(t, tc.expectedProvider, provider, "Global 'provider' flag variable mismatch")
			assert.Equal(t, tc.expectedModel, modelName, "Global 'modelName' flag variable mismatch")
			assert.Equal(t, tc.expectedTemp, temperature, "Global 'temperature' flag variable mismatch")
			assert.Equal(t, tc.expectedTokens, maxTokens, "Global 'maxTokens' flag variable mismatch")

			// 2. Check that the correct LLM constructor was called (by checking which options slice was populated)
			if tc.expectedProvider == "openai" {
				assert.NotNil(t, capturedOpenAIOptions, "newOpenAIFn should have been called for OpenAI provider")
				assert.Nil(t, capturedAnthropicOptions, "newAnthropicFn should NOT have been called for OpenAI provider")
				// Further check if openai.WithModel(tc.expectedModel) was among capturedOpenAIOptions
				// This requires inspecting the functions, which is complex.
				// We rely on the output log for model name confirmation.
			} else if tc.expectedProvider == "anthropic" {
				assert.NotNil(t, capturedAnthropicOptions, "newAnthropicFn should have been called for Anthropic provider")
				assert.Nil(t, capturedOpenAIOptions, "newOpenAIFn should NOT have been called for Anthropic provider")
				// Similar check for anthropic.WithModel(tc.expectedModel)
			}

			// 3. Check the command's output log for correct configuration details
			assert.Contains(t, output, fmt.Sprintf("Provider: %s", tc.expectedProvider), "Output log mismatch for provider")
			assert.Contains(t, output, fmt.Sprintf("Model Name (configured in LLM client): %s", tc.expectedModel), "Output log mismatch for model name")
			if tc.expectedTemp > 0.0 { // Temperature might not be in log if 0
				assert.Contains(t, output, fmt.Sprintf("Temperature: %f", tc.expectedTemp), "Output log mismatch for temperature")
			}
			if tc.expectedTokens > 0 { // Max tokens might not be in log if 0
				assert.Contains(t, output, fmt.Sprintf("Max Tokens: %d", tc.expectedTokens), "Output log mismatch for max tokens")
			}
		})
	}
}
