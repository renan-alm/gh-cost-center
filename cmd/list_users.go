package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listUsersCmd = &cobra.Command{
	Use:   "list-users",
	Short: "List all Copilot license holders",
	Long: `List all GitHub Copilot license holders in the enterprise.

Shows each user with their PRU exception status.

Examples:
  gh cost-center list-users
  gh cost-center list-users -v`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Wire to business logic in later PRs
		fmt.Println("list-users command called")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listUsersCmd)
}
