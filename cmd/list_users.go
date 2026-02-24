package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/renan-alm/gh-cost-center/internal/github"
	"github.com/renan-alm/gh-cost-center/internal/pru"
)

var listUsersCmd = &cobra.Command{
	Use:   "list-users",
	Short: "List all Copilot license holders",
	Long: `List all GitHub Copilot license holders in the enterprise.

Shows each user with their PRU exception status.

Examples:
  gh cost-center list-users
  gh cost-center list-users -v`,
	RunE: runListUsers,
}

func init() {
	rootCmd.AddCommand(listUsersCmd)
}

func runListUsers(_ *cobra.Command, _ []string) error {
	logger := slog.Default()

	// Create GitHub API client.
	client, err := github.NewClient(cfgManager, logger)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	// Initialize PRU manager (needed for exception check).
	mgr := pru.NewManager(cfgManager, logger)

	// Fetch Copilot users.
	users, err := client.GetCopilotUsers()
	if err != nil {
		return fmt.Errorf("fetching copilot users: %w", err)
	}

	// Display users with PRU exception markers.
	fmt.Println("\n=== Copilot License Holders ===")
	fmt.Printf("Total users: %d\n", len(users))
	for _, u := range users {
		marker := ""
		if mgr.IsException(u.Login) {
			marker = " [PRUs Exception]"
		}
		fmt.Printf("- %s%s\n", u.Login, marker)
	}

	return nil
}
