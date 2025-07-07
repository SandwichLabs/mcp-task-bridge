package cmd

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/sandwichlabs/mcp-task-bridge/internal/tui"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [Taskfile]",
	Short: "View the MCP configuration in an interactive TUI.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskfilePath := args[0]
		taskBinPath, _ := cmd.Flags().GetString("task-bin")
		if _, err := os.Stat(taskfilePath); os.IsNotExist(err) {
			slog.Error("Taskfile not found", "path", taskfilePath)
			os.Exit(1)
		}

		inspector, err := inspector.New(
			inspector.WithTaskfile(taskfilePath),
			inspector.WithTaskBin(taskBinPath),
		)
		if err != nil {
			slog.Error("Error creating inspector", "error", err)
			os.Exit(1)
		}
		config, err := inspector.Inspect()
		if err != nil {
			slog.Error("Error inspecting Taskfile", "error", err, "inspectorConfig", inspector)
			os.Exit(1)
		}

		if len(config.Tasks) == 0 {
			fmt.Println("No tasks found in the Taskfile.")
			return
		}

		model := tui.NewModel(config)
		// Initialize Bubble Tea program.
		// It's good practice to use tea.WithOutput(os.Stderr) if you want to log to stdout
		// or if other parts of your app print to stdout.
		// tea.WithAltScreen() provides a better TUI experience.
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(os.Stderr))

		if _, err := p.Run(); err != nil {
			slog.Error("Error running TUI", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	viewCmd.Flags().String("task-bin", "task", "Path to the task binary (default: 'task')")
	rootCmd.AddCommand(viewCmd)
}
