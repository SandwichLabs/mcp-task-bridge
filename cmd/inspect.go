package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [Taskfile]",
	Short: "Inspect a Taskfile and output its MCP configuration.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		taskBinPath, _ := cmd.Flags().GetString("task-bin")
		config, err := inspector.Inspect(taskBinPath, args[0])
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		jsonConfig, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling config to JSON:", err)
			return
		}
		fmt.Println(string(jsonConfig))
	},
}

func init() {
	inspectCmd.Flags().String("task-bin", "task", "Path to the task binary (default: 'task')")
	rootCmd.AddCommand(inspectCmd)
}
