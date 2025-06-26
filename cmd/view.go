package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [Taskfile]",
	Short: "View the MCP configuration in an interactive TUI.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TUI not implemented yet.")
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}
