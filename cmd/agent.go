package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	// react "github.com/tmc/langchaingo/agents/react" // Example if react agent was to be used
)

// Function variables for LLM constructors to allow mocking in tests
var (
	newOpenAIFn    = openai.New
	newAnthropicFn = anthropic.New
)

// taskExecutorTool implements the tools.Tool interface
type taskExecutorTool struct {
	taskName        string
	taskDescription string
	taskUsage       string
	taskfilePath    string
}

func (t *taskExecutorTool) Name() string {
	return t.taskName
}

func (t *taskExecutorTool) Description() string {
	// It's often helpful for the LLM to know how to use the tool, including parameters.
	// Combining description and usage.
	return fmt.Sprintf("%s Usage: %s", t.taskDescription, t.taskUsage)
}

func (t *taskExecutorTool) Call(ctx context.Context, input string) (string, error) {
	slog.Info("Executing tool (task)", "name", t.taskName, "input", input)
	taskCmdArgs := []string{"-t", t.taskfilePath, t.taskName}
	if input != "" {
		taskCmdArgs = append(taskCmdArgs, strings.Fields(input)...) // strings.Fields splits by whitespace
	}
	slog.Debug("Preparing to run command", "command", "task", "args", taskCmdArgs)

	// #nosec G204
	execCmd := exec.CommandContext(ctx, "task", taskCmdArgs...)
	var outbuf, errbuf strings.Builder
	execCmd.Stdout = &outbuf
	execCmd.Stderr = &errbuf

	err := execCmd.Run()
	stdout := strings.TrimSpace(outbuf.String())
	stderr := strings.TrimSpace(errbuf.String())

	if err != nil {
		slog.Error("Error executing task", "task", t.taskName, "error", err, "stdout", stdout, "stderr", stderr)
		return fmt.Sprintf("Error executing task %s: %v. Stderr: %s. Stdout: %s", t.taskName, err, stderr, stdout), nil
	}

	slog.Info("Task executed successfully", "task", t.taskName, "stdout", stdout, "stderr", stderr)
	if stderr != "" {
		return fmt.Sprintf("Stdout:\n%s\nStderr:\n%s", stdout, stderr), nil
	}
	return stdout, nil
}

var (
	provider    string
	modelName   string
	temperature float64
	maxTokens   int
	agentCmd    = &cobra.Command{
		Use:   "agent [Taskfile]",
		Short: "Run a Langchain agent with tools from a Taskfile.",
		Long:  `The agent command configures and runs a Langchain Go REACT agent. Tools are derived from the provided Taskfile.`,
		Args:  cobra.ExactArgs(1),
		Run:   runAgent,
	}
)

func init() {
	agentCmd.Flags().StringVar(&provider, "provider", "anthropic", "LLM provider (e.g., anthropic, openai)")
	agentCmd.Flags().StringVar(&modelName, "model-name", "claude-3-sonnet-20240229", "Name of the model to use")
	agentCmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Sampling temperature for the LLM (0.0-1.0)")
	agentCmd.Flags().IntVar(&maxTokens, "max-tokens", 256, "Maximum number of tokens to generate")
	rootCmd.AddCommand(agentCmd)
}

func runAgent(cmd *cobra.Command, args []string) {
	taskfilePath := args[0]
	slog.Info("Starting agent command", "taskfile", taskfilePath)

	mcpConfig, err := inspector.Inspect(taskfilePath)
	if err != nil {
		slog.Error("Failed to inspect Taskfile", "error", err)
		return
	}
	slog.Info("Successfully inspected Taskfile", "task_count", len(mcpConfig.Tasks))

	var llm llms.Model // Use llms.Model interface
	var llmCallOpts []llms.CallOption

	if temperature > 0.0 { // Only add if set, 0.0 might be default or invalid for some models
		llmCallOpts = append(llmCallOpts, llms.WithTemperature(temperature))
	}
	if maxTokens > 0 { // Only add if set
		llmCallOpts = append(llmCallOpts, llms.WithMaxTokens(maxTokens))
	}

	switch provider {
	case "openai":
		// Assumes openai.New and its options like WithToken, WithModel exist in v0.1.13.
		// This might need adjustment if the API is different (e.g., direct params token, model to New).
		opts := []openai.Option{
			openai.WithToken(getOpenAIToken()),
			openai.WithModel(modelName), // Model name for the client
		}
		llm, err = newOpenAIFn(opts...) // Use the function variable
		if err != nil {
			slog.Error("Failed to initialize OpenAI LLM", "error", err)
			return
		}
		slog.Info("OpenAI LLM client initialized", "configured_model_for_client", modelName)
	case "anthropic":
		opts := []anthropic.Option{
			anthropic.WithToken(getAnthropicToken()),
			anthropic.WithModel(modelName), // Model name for the client
		}
		llm, err = newAnthropicFn(opts...) // Use the function variable
		if err != nil {
			slog.Error("Failed to initialize Anthropic LLM", "error", err)
			return
		}
		slog.Info("Anthropic LLM client initialized", "configured_model_for_client", modelName)
	default:
		slog.Error("Unsupported LLM provider", "provider", provider)
		return
	}

	var langchainTools []tools.Tool
	for _, taskDef := range mcpConfig.Tasks {
		tool := &taskExecutorTool{
			taskName:        taskDef.Name,
			taskDescription: taskDef.Description,
			taskUsage:       taskDef.Usage,
			taskfilePath:    taskfilePath,
		}
		langchainTools = append(langchainTools, tool)
		slog.Debug("Created tool", "name", tool.Name(), "description", tool.Description())
	}

	slog.Info("LLM client and tools prepared.", "llm_type", fmt.Sprintf("%T", llm), "num_tools", len(langchainTools))
	slog.Info("LLM call options prepared (for use during agent execution)", "options_count", len(llmCallOpts))
	for _, opt := range llmCallOpts {
		tempOpts := &llms.CallOptions{}
		opt(tempOpts) // Apply option to see its effect (for logging)
		slog.Debug("LLM Call Opt", "opt_details", fmt.Sprintf("%+v", tempOpts))
	}

	// Agent Execution Logic (Simplified for v0.1.13 compatibility)
	// For a real agent, you would:
	// 1. Construct a specific agent type (e.g., ReAct, Conversational) using the llm and langchainTools.
	//    Example (hypothetical, API for react.NewAgent needs checking for v0.1.13):
	//    myReactAgent, err := react.NewAgent(llm, langchainTools, react.WithLLMCallOptions(llmCallOpts))
	//    if err != nil { slog.Error("Failed to create react agent", "error", err); return }
	// 2. Create an agent.Executor with this agent.
	//    agentExecutor := agents.NewExecutor(myReactAgent)
	// 3. Run the executor with input.
	//    response, err := agentExecutor.Call(context.Background(), map[string]any{"input": "Your question here"})

	// For now, just log the configuration details as the main output.
	fmt.Println("\n--- Agent Configuration (v0.1.13 API Structure) ---")
	fmt.Printf("Provider: %s\n", provider)
	fmt.Printf("Model Name (configured in LLM client): %s\n", modelName)
	fmt.Printf("LLM Call Options (for execution):\n")
	if temperature > 0.0 { fmt.Printf("  Temperature: %f\n", temperature) }
	if maxTokens > 0 { fmt.Printf("  Max Tokens: %d\n", maxTokens) }
	if len(llmCallOpts) == 0 { fmt.Println("  (No specific call options like temp/max_tokens set via flags)")}

	fmt.Println("\nTools:")
	for i, tool := range langchainTools {
		fmt.Printf("  Tool %d:\n", i+1)
		fmt.Printf("    Name: %s\n", tool.Name())
		// Description now includes usage
		fmt.Printf("    Description & Usage: %s\n", tool.Description())
	}
	fmt.Println("--- End of Agent Configuration ---")

	// This satisfies the "declared and not used" for agentExecutor if we don't fully set it up.
	var agentExecutor *agents.Executor
	_ = agentExecutor // Prevent unused variable error.
	slog.Info("Agent components (LLM, Tools, Call Options) are configured. Full agent execution would require specific agent type construction (e.g., ReAct) and use of agents.NewExecutor for v0.1.13.")
}

func getOpenAIToken() string {
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		slog.Warn("OPENAI_API_KEY environment variable not set. Using placeholder.")
		return "sk-your-api-key-placeholder"
	}
	return token
}

func getAnthropicToken() string {
	token := os.Getenv("ANTHROPIC_API_KEY")
	if token == "" {
		slog.Warn("ANTHROPIC_API_KEY environment variable not set. Using placeholder.")
		return "anthropic-api-key-placeholder"
	}
	return token
}
