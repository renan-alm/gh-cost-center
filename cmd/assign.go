package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// assign flags
	assignMode           string
	assignYes            bool
	assignTeams          bool
	assignRepo           bool
	assignUsers          string
	assignIncremental    bool
	assignCreateCC       bool
	assignCreateBudgets  bool
	assignCheckCurrentCC bool
)

var assignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Assign users or repositories to cost centers",
	Long: `Assign users or repositories to cost centers based on the selected mode.

Modes:
  PRU-based (default):   Assigns all Copilot users to cost centers based on
                         PRU exception rules.
  Teams-based (--teams): Assigns users based on GitHub team membership.
  Repository (--repo):   Assigns repos based on custom property values.

The --mode flag controls execution:
  plan  - Preview changes without applying (default)
  apply - Push assignments to GitHub Enterprise

Examples:
  # Preview PRU-based assignments
  gh cost-center assign --mode plan

  # Apply PRU-based assignments (skip confirmation)
  gh cost-center assign --mode apply --yes

  # Preview teams-based assignments
  gh cost-center assign --teams --mode plan

  # Apply teams-based assignments
  gh cost-center assign --teams --mode apply --yes

  # Apply with cost center auto-creation
  gh cost-center assign --mode apply --yes --create-cost-centers

  # Process only new users since last run
  gh cost-center assign --mode apply --yes --incremental

  # Apply repository-based assignments
  gh cost-center assign --repo --mode apply --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Wire to business logic in later PRs
		fmt.Println("assign command called")
		fmt.Printf("  mode:                %s\n", assignMode)
		fmt.Printf("  teams:               %t\n", assignTeams)
		fmt.Printf("  repo:                %t\n", assignRepo)
		fmt.Printf("  yes:                 %t\n", assignYes)
		fmt.Printf("  incremental:         %t\n", assignIncremental)
		fmt.Printf("  create-cost-centers: %t\n", assignCreateCC)
		fmt.Printf("  create-budgets:      %t\n", assignCreateBudgets)
		fmt.Printf("  check-current:       %t\n", assignCheckCurrentCC)
		if assignUsers != "" {
			fmt.Printf("  users:               %s\n", assignUsers)
		}
		return nil
	},
}

func init() {
	assignCmd.Flags().StringVar(&assignMode, "mode", "plan", "execution mode: plan (preview) or apply (push changes)")
	assignCmd.Flags().BoolVarP(&assignYes, "yes", "y", false, "skip confirmation prompt in apply mode")
	assignCmd.Flags().BoolVar(&assignTeams, "teams", false, "enable teams-based assignment mode")
	assignCmd.Flags().BoolVar(&assignRepo, "repo", false, "enable repository-based assignment mode")
	assignCmd.Flags().StringVar(&assignUsers, "users", "", "comma-separated list of specific users to process")
	assignCmd.Flags().BoolVar(&assignIncremental, "incremental", false, "only process users added since last run (PRU mode)")
	assignCmd.Flags().BoolVar(&assignCreateCC, "create-cost-centers", false, "create cost centers if they don't exist")
	assignCmd.Flags().BoolVar(&assignCreateBudgets, "create-budgets", false, "create budgets for new cost centers")
	assignCmd.Flags().BoolVar(&assignCheckCurrentCC, "check-current", false, "check current cost center membership before assigning")

	rootCmd.AddCommand(assignCmd)
}
