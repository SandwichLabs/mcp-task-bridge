package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zac/omcp/internal/server"
)

var rootCmd = &cobra.Command{
	Use:   "omcp [Taskfile]",
	Short: "A CLI to bridge Taskfiles with MCP.",
	Long:  `omcp is a command-line tool that evaluates a Taskfile and exposes its tasks as MCP functions.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		server.Run(args[0])
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
