package cmd

import (
	"fmt"
	"os"

	"github.com/sandwichlabs/mcp-task-bridge/internal/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tmcp [Taskfile]",
	Short: "A CLI to bridge Taskfiles with MCP.",
	Long:  `tmcp is a command-line tool that evaluates a Taskfile and exposes its tasks as MCP functions.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		servername, _ := cmd.Flags().GetString("name")
		if servername == "" {
			servername = "tasks"
		}
		taskBinPath, _ := cmd.Flags().GetString("task-bin")

		server.Run(args[0], taskBinPath, servername)
	},
}

func init() {
	rootCmd.Flags().String("name", "", "Name of the MCP server (default: 'tasks')")
	rootCmd.Flags().String("task-bin", "task", "Path to the task binary (default: 'task')")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
