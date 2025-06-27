package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	// "github.com/tmc/langchaingo/agents" // No longer directly initializing full agent executor here
	// "github.com/tmc/langchaingo/tools" // tools.Tool is used as an interface
)

// mockLLM is a mock implementation of llms.Model for testing.
type mockLLM struct {
	expectedModelName string
	t                 *testing.T
	callFn            func(ctx context.Context, prompt string, options ...llms.CallOption) (string, error)
	generateFn        func(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error)
}

func (m *mockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	if m.callFn != nil {
		return m.callFn(ctx, prompt, options...)
	}
	// Basic verification of options if needed
	appliedOpts := llms.CallOptions{}
	for _, opt := range options {
		opt(&appliedOpts)
	}
	// Can assert properties of appliedOpts if necessary for a specific test
	return "mocked LLM response", nil
}

func (m *mockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, options...)
	}
	appliedOpts := llms.CallOptions{}
	for _, opt := range options {
		opt(&appliedOpts)
	}
	// Can assert properties of appliedOpts if necessary
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: "mocked LLM generated content"},
		},
	}, nil
}

// Stores captured options for openai.New or anthropic.New
var capturedOpenAIOptions []openai.Option
var capturedAnthropicOptions []anthropic.Option

// Mock a successful inspection
func mockInspect(taskfilePath string) (*inspector.MCPConfig, error) {
	return &inspector.MCPConfig{
		Tasks: []inspector.TaskDefinition{
			{Name: "test-task", Description: "A test task", Usage: "task test-task PARAM=value"},
			{Name: "another-task", Description: "Another test task", Usage: "task another-task"},
		},
	}, nil
}

func mockInspectError(_ string) (*inspector.MCPConfig, error) {
	return nil, assert.AnError
}

var originalInspect func(string) (*inspector.MCPConfig, error)
var originalOpenAINew func(...openai.Option) (llms.Model, error) // Changed return type to llms.Model
var originalAnthropicNew func(...anthropic.Option) (llms.Model, error) // Changed return type to llms.Model
var originalGetOpenAIToken func() string
var originalGetAnthropicToken func() string

func setupAgentTest(t *testing.T) {
	originalInspect = inspector.Inspect
	originalOpenAINew = openai.New // This now refers to the actual constructor in agent.go
	originalAnthropicNew = anthropic.New
	originalGetOpenAIToken = getOpenAIToken
	originalGetAnthropicToken = getAnthropicToken

	getOpenAIToken = func() string { return "test-openai-token" }
	getAnthropicToken = func() string { return "test-anthropic-token" }

	// Reset captured options
	capturedOpenAIOptions = nil
	capturedAnthropicOptions = nil

	// Set default flag values for agentCmd (reflecting new defaults)
	provider = "anthropic"
	modelName = "claude-3-sonnet-20240229"
	temperature = 0.7
	maxTokens = 256

	// Mock the LLM constructors
	openai.New = func(opts ...openai.Option) (llms.Model, error) {
		capturedOpenAIOptions = opts
		// Check if WithModel option was passed and extract model name for assertion
		var mName string
		tempLLM := &openai.LLM{} // Use actual LLM struct to apply options for inspection
		for _, opt := range opts {
			// This is tricky as Option is unexported type in some versions.
			// We assume applying to a real struct works for inspection.
			// If openai.Option is an interface, this might need a more complex mock.
			// For v0.1.13, openai.Option is `func(*openai.LLM)`.
			opt(tempLLM)
		}
		mName = tempLLM.ModelName // Assuming ModelName field exists or WithModel sets it
		return &mockLLM{t: t, expectedModelName: mName}, nil
	}
	anthropic.New = func(opts ...anthropic.Option) (llms.Model, error) {
		capturedAnthropicOptions = opts
		var mName string
		tempLLM := &anthropic.LLM{} // Use actual LLM struct
		for _, opt := range opts {
			opt(tempLLM)
		}
		mName = tempLLM.ModelName
		return &mockLLM{t: t, expectedModelName: mName}, nil
	}
}

func teardownAgentTest(t *testing.T) {
	inspector.Inspect = originalInspect
	openai.New = originalOpenAINew
	anthropic.New = originalAnthropicNew
	getOpenAIToken = originalGetOpenAIToken
	getAnthropicToken = originalGetAnthropicToken
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestAgentCommand_RunSuccess_AnthropicDefault(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)

	// Run with default (Anthropic)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--temperature", "0.5", "--max-tokens", "150")
	assert.NoError(t, err)

	assert.Contains(t, output, "Provider: anthropic")
	assert.Contains(t, output, "Model Name (configured in LLM client): claude-3-sonnet-20240229")
	assert.Contains(t, output, "Temperature: 0.500000")
	assert.Contains(t, output, "Max Tokens: 150")
	assert.Contains(t, output, "Name: test-task")
	assert.Contains(t, output, "Description & Usage: A test task Usage: task test-task PARAM=value")
	assert.Contains(t, output, "Name: another-task")

	// Verify anthropic.New was called and captured options
	assert.NotNil(t, capturedAnthropicOptions, "anthropic.New should have been called")
	assert.Nil(t, capturedOpenAIOptions, "openai.New should NOT have been called")

	// Check that WithModel was passed to anthropic.New with the default model
	foundModelOpt := false
	tempAnthropicLLM := &anthropic.LLM{}
	for _, opt := range capturedAnthropicOptions {
		opt(tempAnthropicLLM)
	}
	if tempAnthropicLLM.ModelName == "claude-3-sonnet-20240229" {
		foundModelOpt = true
	}
	assert.True(t, foundModelOpt, "anthropic.WithModel option for default model not found or incorrect")
}

func TestAgentCommand_RunSuccess_OpenAI(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect

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

	assert.NotNil(t, capturedOpenAIOptions, "openai.New should have been called")
	assert.Nil(t, capturedAnthropicOptions, "anthropic.New should NOT have been called")

	foundModelOpt := false
	tempOpenAILLM := &openai.LLM{}
	for _, opt := range capturedOpenAIOptions {
		opt(tempOpenAILLM)
	}
	if tempOpenAILLM.ModelName == "gpt-4-test" { // Assuming ModelName is the field set by WithModel
		foundModelOpt = true
	}
	assert.True(t, foundModelOpt, "openai.WithModel option for gpt-4-test not found or incorrect")
}

func TestAgentCommand_InspectError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)
	inspector.Inspect = mockInspectError
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml")
	assert.Contains(t, output, "Failed to inspect Taskfile")
}

func TestAgentCommand_OpenAIInitError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)
	inspector.Inspect = mockInspect
	openai.New = func(opts ...openai.Option) (llms.Model, error) { // Mock returns llms.Model
		return nil, assert.AnError
	}
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "openai")
	assert.Contains(t, output, "Failed to initialize OpenAI LLM")
}

func TestAgentCommand_AnthropicInitError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)
	inspector.Inspect = mockInspect
	anthropic.New = func(opts ...anthropic.Option) (llms.Model, error) { // Mock returns llms.Model
		return nil, assert.AnError
	}
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "anthropic")
	assert.Contains(t, output, "Failed to initialize Anthropic LLM")
}

func TestAgentCommand_UnsupportedProvider(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)
	inspector.Inspect = mockInspect
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, _ := executeCommand(testRootCmd, "agent", "dummy.yml", "--provider", "unknown")
	assert.Contains(t, output, "Unsupported LLM provider")
}

func TestTaskExecutorTool_Call(t *testing.T) {
	// This test now focuses on the tool's Call method directly.
	// More complex mocking of exec.Command would be needed for full validation.
	tool := &taskExecutorTool{
		taskName:        "test-exec-task",
		taskDescription: "A task for execution test",
		taskUsage:       "task test-exec-task INPUT=value",
		taskfilePath:    "TestTaskfile.yml", // Needs a dummy or mock Taskfile
	}

	// Create a dummy Taskfile for this test
	dummyTaskContent := `
version: '3'
tasks:
  test-exec-task:
    cmds:
      - echo "Executing test-exec-task with $INPUT"
    vars:
      INPUT: ""
`
	dummyTaskfilePath := "TestTaskfile.ymlForToolCall.yml"
	err := os.WriteFile(dummyTaskfilePath, []byte(dummyTaskContent), 0600)
	assert.NoError(t, err)
	defer os.Remove(dummyTaskfilePath)
	tool.taskfilePath = dummyTaskfilePath // Point to the created taskfile

	ctx := context.Background()
	// Test case 1: No input
	// output, err := tool.Call(ctx, "")
	// assert.NoError(t, err)
	// assert.Contains(t, output, "Executing test-exec-task with ")

	// Test case 2: With input
	output, err := tool.Call(ctx, "INPUT=hello")
	assert.NoError(t, err)
	assert.Contains(t, output, "Executing test-exec-task with hello")

	// Test case 3: Simulating an error (e.g., task not found - harder to do without complex exec mock)
	// For now, this aspect is implicitly covered by agent.go's error handling.
}


func TestGetOpenAIToken(t *testing.T) {
	original := getOpenAIToken
	defer func() { getOpenAIToken = original }()

	os.Setenv("OPENAI_API_KEY", "env-openai-key")
	assert.Equal(t, "env-openai-key", getOpenAIToken())
	os.Unsetenv("OPENAI_API_KEY")
	assert.Equal(t, "sk-your-api-key-placeholder", getOpenAIToken())
}

func TestGetAnthropicToken(t *testing.T) {
	original := getAnthropicToken
	defer func() { getAnthropicToken = original }()

	os.Setenv("ANTHROPIC_API_KEY", "env-anthropic-key")
	assert.Equal(t, "env-anthropic-key", getAnthropicToken())
	os.Unsetenv("ANTHROPIC_API_KEY")
	assert.Equal(t, "anthropic-api-key-placeholder", getAnthropicToken())
}

func TestAgentCmdFlags(t *testing.T) {
	setupAgentTest(t) // This sets up mocks for openai.New and anthropic.New
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect // Ensure inspect doesn't fail

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testCases := []struct {
		name             string
		args             []string
		expectedProvider string
		expectedModel    string
		expectedTemp     float64
		expectedTokens   int
		assertLLMOptions func(t *testing.T, expectedModel string) // For checking options passed to New
	}{
		{
			"Defaults (Anthropic)",
			[]string{"agent", dummyTaskfile},
			"anthropic", "claude-3-sonnet-20240229", 0.7, 256,
			func(t *testing.T, expectedModel string) {
				assert.NotNil(t, capturedAnthropicOptions)
				tempLLM := &anthropic.LLM{}
				for _, opt := range capturedAnthropicOptions { opt(tempLLM) }
				assert.Equal(t, expectedModel, tempLLM.ModelName)
			},
		},
		{
			"Custom Anthropic",
			[]string{"agent", dummyTaskfile, "--model-name", "claude-opus-test", "--temperature", "0.3", "--max-tokens", "600"},
			"anthropic", "claude-opus-test", 0.3, 600,
			func(t *testing.T, expectedModel string) {
				assert.NotNil(t, capturedAnthropicOptions)
				tempLLM := &anthropic.LLM{}
				for _, opt := range capturedAnthropicOptions { opt(tempLLM) }
				assert.Equal(t, expectedModel, tempLLM.ModelName)
			},
		},
		{
			"Custom OpenAI",
			[]string{"agent", dummyTaskfile, "--provider", "openai", "--model-name", "gpt-4-test", "--temperature", "0.2", "--max-tokens", "500"},
			"openai", "gpt-4-test", 0.2, 500,
			func(t *testing.T, expectedModel string) {
				assert.NotNil(t, capturedOpenAIOptions)
				tempLLM := &openai.LLM{}
				for _, opt := range capturedOpenAIOptions { opt(tempLLM) }
				assert.Equal(t, expectedModel, tempLLM.ModelName) // Assuming ModelName field after WithModel
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global flag variables to Cobra defaults before each test run of executeCommand
			// These are the defaults defined in agentCmd.Flags()
			provider = "anthropic"
			modelName = "claude-3-sonnet-20240229"
			temperature = 0.7
			maxTokens = 256

			// Reset captured options for each test case
			capturedOpenAIOptions = nil
			capturedAnthropicOptions = nil

			testRootCmd := &cobra.Command{Use: "tmcp"}
			testRootCmd.AddCommand(agentCmd)
			output, err := executeCommand(testRootCmd, tc.args...)
			assert.NoError(t, err)

			// Check that the global flag variables were updated by Cobra
			assert.Equal(t, tc.expectedProvider, provider, "Provider flag variable mismatch")
			assert.Equal(t, tc.expectedModel, modelName, "ModelName flag variable mismatch")
			assert.Equal(t, tc.expectedTemp, temperature, "Temperature flag variable mismatch")
			assert.Equal(t, tc.expectedTokens, maxTokens, "MaxTokens flag variable mismatch")

			// Check that the correct New function was called with correct options
			tc.assertLLMOptions(t, tc.expectedModel)

			// Check output string for temperature and max_tokens as they are part of llms.CallOptions now
			// and logged differently.
			assert.Contains(t, output, fmt.Sprintf("Temperature: %f", tc.expectedTemp))
			assert.Contains(t, output, fmt.Sprintf("Max Tokens: %d", tc.expectedTokens))
			assert.Contains(t, output, fmt.Sprintf("Model Name (configured in LLM client): %s", tc.expectedModel))

		})
	}
}
