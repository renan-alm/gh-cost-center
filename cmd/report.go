package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/renan-alm/gh-cost-center/internal/cache"
	"github.com/renan-alm/gh-cost-center/internal/customprop"
	"github.com/renan-alm/gh-cost-center/internal/github"
	"github.com/renan-alm/gh-cost-center/internal/pru"
	"github.com/renan-alm/gh-cost-center/internal/teams"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate cost center summary report",
	Long: `Generate and display a cost center summary report.

Shows per-cost-center user counts and assignment breakdown.
The report type is determined by cost_center.mode in config.yaml.

Examples:
  gh cost-center report`,
	RunE: runReport,
}

func init() {
	rootCmd.AddCommand(reportCmd)
}

func runReport(_ *cobra.Command, _ []string) error {
	switch cfgManager.CostCenterMode {
	case "teams":
		return runTeamsReport()
	case "custom-prop":
		return runCustomPropReport()
	default:
		// "users" (PRU) is the default
	}

	logger := slog.Default()

	// Create GitHub API client.
	client, err := github.NewClient(cfgManager, logger)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	// Initialize PRU manager.
	mgr := pru.NewManager(cfgManager, logger)

	// Fetch Copilot users.
	users, err := client.GetCopilotUsers()
	if err != nil {
		return fmt.Errorf("fetching copilot users: %w", err)
	}

	// Generate and display summary.
	summary := mgr.GenerateSummary(users)

	fmt.Println("\n=== Cost Center Summary ===")
	logger.Info("Cost Center Assignment Summary")
	for cc, count := range summary {
		fmt.Printf("%s: %d users\n", cc, count)
		logger.Info("Cost center", "id", cc, "users", count)
	}

	return nil
}

// runTeamsReport generates a teams-aware cost center report.
func runTeamsReport() error {
	logger := slog.Default()

	client, err := github.NewClient(cfgManager, logger)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	mgr := teams.NewManager(cfgManager, client, logger)

	summary, err := mgr.GenerateSummary()
	if err != nil {
		return fmt.Errorf("generating teams summary: %w", err)
	}

	summary.Print(cfgManager.Enterprise)

	return nil
}

// runCustomPropReport generates a custom-property cost center summary.
func runCustomPropReport() error {
	logger := slog.Default()

	if len(cfgManager.Organizations) == 0 {
		return fmt.Errorf("custom-prop mode requires at least one organization in github.organizations config")
	}
	org := cfgManager.Organizations[0]

	client, err := github.NewClient(cfgManager, logger)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	cc, cacheErr := cache.New("", logger)
	if cacheErr == nil {
		client.SetCache(cc)
	}

	cpMgr, err := customprop.NewManager(cfgManager, client, logger)
	if err != nil {
		return fmt.Errorf("initializing custom-property manager: %w", err)
	}

	summary, err := cpMgr.GenerateSummary(org)
	if err != nil {
		return fmt.Errorf("generating custom-property summary: %w", err)
	}

	summary.Print()

	return nil
}
