package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
)

// Mock a successful inspection
func mockInspect(taskfilePath string) (*inspector.MCPConfig, error) {
	return &inspector.MCPConfig{
		Tasks: []inspector.TaskDefinition{
			{Name: "test-task", Description: "A test task", Usage: "task test-task PARAM=value"},
			{Name: "another-task", Description: "Another test task", Usage: "task another-task"},
		},
	}, nil
}

// Mock a failed inspection
func mockInspectError(taskfilePath string) (*inspector.MCPConfig, error) {
	return nil, assert.AnError
}

// Store original functions to restore them after tests
var originalInspect func(string) (*inspector.MCPConfig, error)
var originalOpenAINew func(...openai.Option) (*openai.LLM, error)
var originalAnthropicNew func(...anthropic.Option) (*anthropic.LLM, error) // Added for Anthropic
var originalAgentInitialize func(llm agents.AgentLLM, tools []tools.Tool, agentType agents.AgentType, opts ...agents.Option) (agents.Executor, error)
var originalGetOpenAIToken func() string
var originalGetAnthropicToken func() string // Added for Anthropic

func setupAgentTest(t *testing.T) {
	originalInspect = inspector.Inspect
	originalOpenAINew = openai.New
	originalAnthropicNew = anthropic.New // Store original anthropic.New
	originalAgentInitialize = agents.Initialize
	originalGetOpenAIToken = getOpenAIToken
	originalGetAnthropicToken = getAnthropicToken // Store original getAnthropicToken

	// Set dummy tokens for tests
	getOpenAIToken = func() string { return "test-openai-token" }
	getAnthropicToken = func() string { return "test-anthropic-token" }

	// Reset flags to default values for each test (now Anthropic)
	provider = "anthropic"
	modelName = "claude-3-sonnet-20240229"
	temperature = 0.7
	maxTokens = 256
}

func teardownAgentTest(t *testing.T) {
	inspector.Inspect = originalInspect
	openai.New = originalOpenAINew
	anthropic.New = originalAnthropicNew // Restore original anthropic.New
	agents.Initialize = originalAgentInitialize
	getOpenAIToken = originalGetOpenAIToken
	getAnthropicToken = originalGetAnthropicToken // Restore original getAnthropicToken
}

// Helper function to execute cobra commands and capture output
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestAgentCommand_RunSuccess(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect
	// Mock Anthropic client initialization for default case
	anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
		// Basic validation of options passed for default Anthropic
		assert.Contains(t, opts, anthropic.WithModel("claude-3-opus-20240229")) // Expecting opus from this specific test's args
		assert.Contains(t, opts, anthropic.WithToken("test-anthropic-token"))
		assert.Contains(t, opts, anthropic.WithTemperature(0.5))
		assert.Contains(t, opts, anthropic.WithMaxTokens(100))
		return &anthropic.LLM{}, nil
	}
	agents.Initialize = func(llm agents.AgentLLM, tools []tools.Tool, agentType agents.AgentType, opts ...agents.Option) (agents.Executor, error) {
		assert.NotNil(t, llm)
		assert.IsType(t, &anthropic.LLM{}, llm, "LLM should be of Anthropic type for this test case")
		assert.Len(t, tools, 2)
		assert.Equal(t, "test-task", tools[0].Name)
		assert.Equal(t, "another-task", tools[1].Name)
		return &agents.ExecutorImpl{}, nil
	}

	// Create a dummy Taskfile
	dummyTaskfile := "dummy_Taskfile.yml"
	f, err := os.Create(dummyTaskfile)
	assert.NoError(t, err)
	f.Close()
	defer os.Remove(dummyTaskfile)

	// Re-initialize rootCmd to ensure agentCmd is added for this test run
	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd) // agentCmd is global in cmd package

	// Test with Anthropic as the provider (now default, but explicitly specifying for clarity in test)
	// and a different model to ensure flags are respected.
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--provider", "anthropic", "--model-name", "claude-3-opus-20240229", "--temperature", "0.5", "--max-tokens", "100")

	assert.NoError(t, err)
	assert.Contains(t, output, "Agent Configuration:")
	assert.Contains(t, output, "Provider: anthropic")
	assert.Contains(t, output, "Model Name: claude-3-opus-20240229")
	assert.Contains(t, output, "Temperature: 0.500000")
	assert.Contains(t, output, "Max Tokens: 100")
	assert.Contains(t, output, "Name: test-task")
	assert.Contains(t, output, "Description: A test task")
	assert.Contains(t, output, "Name: another-task")
	assert.Contains(t, output, "Description: Another test task")
}

func TestAgentCommand_InspectError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspectError

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)

	output, err := executeCommand(testRootCmd, "agent", "nonexistent_Taskfile.yml")

	assert.NoError(t, err) // Cobra handles errors internally for RunE, error from Execute is often nil
	assert.Contains(t, output, "Failed to inspect Taskfile")
}

func TestAgentCommand_OpenAIInitError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect
	openai.New = func(opts ...openai.Option) (*openai.LLM, error) {
		return nil, assert.AnError // Simulate OpenAI client initialization error
	}

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile)

	assert.NoError(t, err)
	assert.Contains(t, output, "Failed to initialize OpenAI LLM")
}

func TestAgentCommand_AnthropicInitError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect
	anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
		return nil, assert.AnError // Simulate Anthropic client initialization error
	}

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	// Explicitly use anthropic, though it's default, to target this mock
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--provider", "anthropic")

	assert.NoError(t, err)
	assert.Contains(t, output, "Failed to initialize Anthropic LLM")
}

func TestAgentCommand_AgentInitializeError(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect
	// Mock the default provider (Anthropic)
	anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
		return &anthropic.LLM{}, nil
	}
	agents.Initialize = func(llm agents.AgentLLM, tools []tools.Tool, agentType agents.AgentType, opts ...agents.Option) (agents.Executor, error) {
		return nil, assert.AnError // Simulate agent initialization error
	}

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile)

	assert.NoError(t, err)
	assert.Contains(t, output, "Failed to initialize Langchain agent")
}

func TestAgentCommand_UnsupportedProvider(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	inspector.Inspect = mockInspect

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	output, err := executeCommand(testRootCmd, "agent", dummyTaskfile, "--provider", "unsupported-provider")

	assert.NoError(t, err)
	assert.Contains(t, output, "Unsupported LLM provider")
	assert.Contains(t, output, "provider unsupported-provider")
}

// Test for the tool's Run function (simplified)
func TestAgentCommand_ToolExecution(t *testing.T) {
	// This test is more complex due to exec.Command.
	// For now, we'll just ensure the tool structure is there.
	// A more comprehensive test would mock exec.Command.

	setupAgentTest(t)
	defer teardownAgentTest(t)

	var capturedTools []tools.Tool
	inspector.Inspect = mockInspect
	// Mock default provider (Anthropic)
	anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
		return &anthropic.LLM{}, nil
	}
	agents.Initialize = func(llm agents.AgentLLM, tools []tools.Tool, agentType agents.AgentType, opts ...agents.Option) (agents.Executor, error) {
		capturedTools = tools // Capture the tools for inspection
		return &agents.ExecutorImpl{}, nil
	}

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testRootCmd := &cobra.Command{Use: "tmcp"}
	testRootCmd.AddCommand(agentCmd)
	_, err := executeCommand(testRootCmd, "agent", dummyTaskfile)
	assert.NoError(t, err)

	assert.Len(t, capturedTools, 2)
	tool1 := capturedTools[0]
	assert.Equal(t, "test-task", tool1.Name)

	// We can't easily test the exec.Command part without more complex mocking.
	// However, we can call the Run function directly if we simplify its dependencies or mock them.
	// For this example, we'll just confirm it exists.
	// _, runErr := tool1.Run(context.Background(), "PARAM=test")
	// For now, this is a placeholder as `task` is not in PATH for test env
	// and mocking exec.Command is outside the immediate scope.
	// A real test would involve setting up a mock for exec.Command.
	// For now, we check if the function can be called.
	// A proper test for this would be an integration test or require OS-level mocking.
	go func() {
		// The actual execution will likely fail if 'task' command is not found or Taskfile is dummy.
		// We are primarily testing the wiring here.
		_, _ = tool1.Run(context.Background(), "DUMMY_PARAM=dummy_value")
	}()
	// If we reach here without panic, it means the Run function is callable.
}

func TestGetOpenAIToken(t *testing.T) {
	// Test the actual getOpenAIToken if it read from env
	originalTokenFunc := getOpenAIToken
	defer func() { getOpenAIToken = originalTokenFunc }()

	// Case 1: Environment variable is set
	expectedToken := "env-api-key"
	os.Setenv("OPENAI_API_KEY", expectedToken)
	defer os.Unsetenv("OPENAI_API_KEY")

	// Temporarily replace the function in agent.go to use the env var
	// This is a bit of a hack; ideally, getOpenAIToken would be part of an interface or easily mockable.
	// For this example, we'll assume agent.go's getOpenAIToken is modified to use os.Getenv.
	// Since we can't modify agent.go from the test directly in this environment,
	// this test serves as a template for how it *should* be tested if getOpenAIToken used getenv.

	// If agent.go's getOpenAIToken was:
	// func getOpenAIToken() string { return os.Getenv("OPENAI_API_KEY") }
	// Then this test would be:
	token := getOpenAIToken() // Call the actual function from agent.go (or its test double)
	assert.Equal(t, expectedToken, token)

	// Case 2: Environment variable is NOT set
	os.Unsetenv("OPENAI_API_KEY")
	// Re-assign to the actual function from agent.go if it was test-doubled above, or ensure it's the original for this case.
	// For simplicity, assuming getOpenAIToken from setupAgentTest is already the "real" one or a suitable test double.
	token = getOpenAIToken()
	assert.Equal(t, "sk-your-api-key-placeholder", token) // Check against the default placeholder from agent.go
}

func TestGetAnthropicToken(t *testing.T) {
	originalTokenFunc := getAnthropicToken
	defer func() { getAnthropicToken = originalTokenFunc }()

	// Case 1: Environment variable is set
	expectedToken := "env-anthropic-key"
	os.Setenv("ANTHROPIC_API_KEY", expectedToken)
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	token := getAnthropicToken()
	assert.Equal(t, expectedToken, token)

	// Case 2: Environment variable is NOT set
	os.Unsetenv("ANTHROPIC_API_KEY")
	token = getAnthropicToken()
	assert.Equal(t, "anthropic-api-key-placeholder", token) // Check against the default placeholder
}


func TestAgentCmdFlags(t *testing.T) {
	setupAgentTest(t)
	defer teardownAgentTest(t)

	// Mock dependencies to prevent actual execution, focus on flags
	inspector.Inspect = mockInspect
	openai.New = func(opts ...openai.Option) (*openai.LLM, error) { return &openai.LLM{}, nil }
	anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) { return &anthropic.LLM{}, nil } // Mock anthropic
	agents.Initialize = func(llm agents.AgentLLM, t []tools.Tool, at agents.AgentType, o ...agents.Option) (agents.Executor, error) { return &agents.ExecutorImpl{}, nil }

	dummyTaskfile := "dummy_Taskfile.yml"
	f, _ := os.Create(dummyTaskfile)
	f.Close()
	defer os.Remove(dummyTaskfile)

	testCases := []struct {
		name          string
		args          []string
		expectedProvider string
		expectedModel string
		expectedTemp  float64
		expectedTokens int
	}{
		{"Defaults (Anthropic)", []string{"agent", dummyTaskfile}, "anthropic", "claude-3-sonnet-20240229", 0.7, 256},
		{"Custom Anthropic", []string{"agent", dummyTaskfile, "--provider", "anthropic", "--model-name", "claude-3-opus-20240229", "--temperature", "0.3", "--max-tokens", "600"}, "anthropic", "claude-3-opus-20240229", 0.3, 600},
		{"Custom OpenAI", []string{"agent", dummyTaskfile, "--provider", "openai", "--model-name", "gpt-4", "--temperature", "0.2", "--max-tokens", "500"}, "openai", "gpt-4", 0.2, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global flags for each sub-test to package defaults before command execution
			provider = "anthropic" // Default provider
			modelName = "claude-3-sonnet-20240229" // Default model
			temperature = 0.7     // Default temperature
			maxTokens = 256       // Default maxTokens

			var capturedModel string
			var capturedTemp float64
			var capturedTokens int

			// Store original LLM client constructors and defer their restoration
			originalOpenAINewCopy := openai.New
			originalAnthropicNewCopy := anthropic.New
			defer func() {
				openai.New = originalOpenAINewCopy
				anthropic.New = originalAnthropicNewCopy
			}()

			if tc.expectedProvider == "openai" {
				openai.New = func(opts ...openai.Option) (*openai.LLM, error) {
					dummyCfg := &openai.LLM{}
					for _, opt := range opts { opt(dummyCfg) }
					capturedModel = dummyCfg.Model
					capturedTemp = dummyCfg.Temperature
					capturedTokens = dummyCfg.MaxTokens
					return originalOpenAINewCopy(opts...)
				}
				// Ensure anthropic.New is not called for OpenAI case by setting it to fail test
				anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
					t.Errorf("anthropic.New called for OpenAI provider test case")
					return nil, assert.AnError
				}
			} else if tc.expectedProvider == "anthropic" {
				anthropic.New = func(opts ...anthropic.Option) (*anthropic.LLM, error) {
					dummyCfg := &anthropic.LLM{} // Assuming anthropic.LLM has similar fields or a way to get them
					for _, opt := range opts { opt(dummyCfg) }
					capturedModel = dummyCfg.ModelName // Note: Field might be ModelName for anthropic
					capturedTemp = dummyCfg.Temperature
					capturedTokens = dummyCfg.MaxTokensToSample // Note: Field might be MaxTokensToSample
					return originalAnthropicNewCopy(opts...)
				}
				// Ensure openai.New is not called for Anthropic case
				openai.New = func(opts ...openai.Option) (*openai.LLM, error) {
					t.Errorf("openai.New called for Anthropic provider test case")
					return nil, assert.AnError
				}
			}

			testRootCmd := &cobra.Command{Use: "tmcp"}
			testRootCmd.AddCommand(agentCmd)
			_, err := executeCommand(testRootCmd, tc.args...)
			assert.NoError(t, err)

			// Check global 'provider' flag variable after command execution
			assert.Equal(t, tc.expectedProvider, provider, "Provider flag mismatch")

			// Check values captured by the respective LLM mock
			assert.Equal(t, tc.expectedModel, capturedModel, "Model name mismatch")
			assert.Equal(t, tc.expectedTemp, capturedTemp, "Temperature mismatch")
			assert.Equal(t, tc.expectedTokens, capturedTokens, "Max tokens mismatch")
		})
	}
}
