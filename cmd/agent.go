package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
)

var (
	provider   string
	modelName  string
	temperature float64
	maxTokens  int
	agentCmd = &cobra.Command{
		Use:   "agent [Taskfile]",
		Short: "Run a Langchain agent with tools from a Taskfile.",
		Long:  `The agent command configures and runs a Langchain Go REACT agent. Tools are derived from the provided Taskfile.`,
		Args:  cobra.ExactArgs(1),
		Run:   runAgent,
	}
)

func init() {
	agentCmd.Flags().StringVar(&provider, "provider", "openai", "LLM provider (e.g., openai, anthropic)")
	agentCmd.Flags().StringVar(&modelName, "model-name", "gpt-3.5-turbo", "Name of the model to use")
	agentCmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Sampling temperature for the LLM")
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

	// Initialize LLM based on provider
	// For now, only OpenAI is supported as an example
	var llm agents.AgentLLM
	if provider == "openai" {
		openAIOptions := []openai.Option{
			openai.WithModel(modelName),
			openai.WithToken(getOpenAIToken()), // Helper function to get API token
		}
		// Optional parameters, only add if they are not default,
		// as langchaingo might have its own defaults or ways to handle zero values.
		if temperature != 0.0 { // Assuming 0.0 is not a valid desired temperature and indicates default
			openAIOptions = append(openAIOptions, openai.WithTemperature(temperature))
		}
		if maxTokens != 0 { // Assuming 0 indicates default
			openAIOptions = append(openAIOptions, openai.WithMaxTokens(maxTokens))
		}

		llm, err = openai.New(openAIOptions...)
		if err != nil {
			slog.Error("Failed to initialize OpenAI LLM", "error", err)
			return
		}
		slog.Info("OpenAI LLM initialized", "model", modelName, "temperature", temperature, "max_tokens", maxTokens)
	} else {
		slog.Error("Unsupported LLM provider", "provider", provider)
		// For now, we exit or handle the error. In the future, more providers can be added.
		return
	}

	// Create Langchain tools from MCPConfig
	var langchainTools []tools.Tool
	for _, task := range mcpConfig.Tasks {
		currentTask := task // Capture range variable
		tool := tools.Tool{
			Name:        currentTask.Name,
			Description: currentTask.Description,
			Run: func(ctx context.Context, input string) (string, error) {
				slog.Info("Executing tool", "name", currentTask.Name, "input", input)
				// This is a simplified execution.
				// Input parsing and mapping to task parameters would be needed here.
				// For now, we assume input is a string of params like "PARAM1=value1 PARAM2=value2"
				// or that the task doesn't require parameters if input is empty.

				taskCmdArgs := []string{"-t", taskfilePath, currentTask.Name}
				if input != "" {
					// A more robust way to parse and pass parameters is needed.
					// This simple split might not cover all edge cases.
					taskCmdArgs = append(taskCmdArgs, strings.Split(input, " ")...)
				}

				slog.Debug("Preparing to run command", "command", "task", "args", taskCmdArgs)

				// #nosec G204
				execCmd := exec.CommandContext(ctx, "task", taskCmdArgs...)
				var outbuf, errbuf strings.Builder
				execCmd.Stdout = &outbuf
				execCmd.Stderr = &errbuf

				err := execCmd.Run()
				stdout := outbuf.String()
				stderr := errbuf.String()

				if err != nil {
					slog.Error("Error executing task", "task", currentTask.Name, "error", err, "stdout", stdout, "stderr", stderr)
					return fmt.Sprintf("error executing task %s: %v. Stderr: %s", currentTask.Name, err, stderr), nil
				}
				slog.Info("Task executed successfully", "task", currentTask.Name, "stdout", stdout)
				return stdout, nil
			},
		}
		langchainTools = append(langchainTools, tool)
		slog.Debug("Created tool", "name", tool.Name, "description", tool.Description)
	}

	// Instantiate the Langchain Go REACT agent
	agentExecutor, err := agents.Initialize(
		llm,
		langchainTools,
		agents.ZeroShotReactDescription, // Using a standard agent type
		agents.WithMaxIterations(5),      // Example agent option
	)
	if err != nil {
		slog.Error("Failed to initialize Langchain agent", "error", err)
		return
	}

	slog.Info("Langchain REACT agent initialized successfully")

	// Log the agent's configuration
	fmt.Println("Agent Configuration:")
	fmt.Printf("  Provider: %s\n", provider)
	fmt.Printf("  Model Name: %s\n", modelName)
	fmt.Printf("  Temperature: %f\n", temperature)
	fmt.Printf("  Max Tokens: %d\n", maxTokens)
	fmt.Println("  Tools:")
	for _, tool := range langchainTools {
		fmt.Printf("    - Name: %s\n", tool.Name)
		fmt.Printf("      Description: %s\n", tool.Description)
	}

	// For now, we just log the config.
	// Actual agent execution with a prompt would go here.
	// e.g., response, err := agentExecutor.Run(context.Background(), "Some input prompt for the agent")
	slog.Info("Agent configured. For now, logging configuration only.")
}

// getOpenAIToken is a placeholder for securely retrieving the OpenAI API token.
// Implement this according to your security best practices (e.g., environment variables).
func getOpenAIToken() string {
	// Example: return os.Getenv("OPENAI_API_KEY")
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		slog.Warn("OPENAI_API_KEY environment variable not set. Using placeholder. Please configure it for actual use.")
		return "sk-your-api-key-placeholder" // Placeholder if not set
	}
	return token
}
