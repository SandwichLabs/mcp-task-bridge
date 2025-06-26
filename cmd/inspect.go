package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zac/omcp/internal/inspector"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [Taskfile]",
	Short: "Inspect a Taskfile and output its MCP configuration.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config, err := inspector.Inspect(args[0])
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
	rootCmd.AddCommand(inspectCmd)
}
