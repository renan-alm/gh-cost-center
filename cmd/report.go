package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var reportTeams bool

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate cost center summary report",
	Long: `Generate and display a cost center summary report.

Shows per-cost-center user counts and assignment breakdown.
Use --teams for teams-aware reporting.

Examples:
  gh cost-center report
  gh cost-center report --teams`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Wire to business logic in later PRs
		fmt.Println("report command called")
		fmt.Printf("  teams: %t\n", reportTeams)
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportTeams, "teams", false, "generate teams-aware report")

	rootCmd.AddCommand(reportCmd)
}
