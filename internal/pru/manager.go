// Package pru implements PRU-based cost center assignment — the default mode.
//
// The logic is simple: every Copilot user is assigned to one of two cost
// centers.  Users on the PRU exception list go to the "PRU-allowed" cost
// center; everyone else goes to the "no-PRU" cost center.
package pru

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/renan-alm/gh-cost-center/internal/config"
	"github.com/renan-alm/gh-cost-center/internal/github"
)

// Manager handles PRU-based cost center assignment.
type Manager struct {
	noPRUCCID      string
	pruAllowedCCID string
	exceptions     map[string]bool // set of exception logins (lower-cased)
	log            *slog.Logger
}

// NewManager creates a PRU manager from the loaded configuration.
func NewManager(cfg *config.Manager, logger *slog.Logger) *Manager {
	exceptions := make(map[string]bool, len(cfg.PRUsExceptionUsers))
	for _, u := range cfg.PRUsExceptionUsers {
		exceptions[strings.ToLower(u)] = true
	}

	logger.Info("Initialized PRU manager",
		"exception_users", len(exceptions),
		"no_pru_cc", cfg.NoPRUsCostCenterID,
		"pru_allowed_cc", cfg.PRUsAllowedCostCenterID,
	)

	return &Manager{
		noPRUCCID:      cfg.NoPRUsCostCenterID,
		pruAllowedCCID: cfg.PRUsAllowedCostCenterID,
		exceptions:     exceptions,
		log:            logger,
	}
}

// SetCostCenterIDs updates the cost center IDs at runtime (e.g. after
// auto-creation resolves placeholders into real UUIDs).
func (m *Manager) SetCostCenterIDs(noPRU, pruAllowed string) {
	m.noPRUCCID = noPRU
	m.pruAllowedCCID = pruAllowed
	m.log.Info("Updated cost center IDs", "no_pru", noPRU, "pru_allowed", pruAllowed)
}

// NoPRUCCID returns the current no-PRU cost center ID.
func (m *Manager) NoPRUCCID() string { return m.noPRUCCID }

// PRUAllowedCCID returns the current PRU-allowed cost center ID.
func (m *Manager) PRUAllowedCCID() string { return m.pruAllowedCCID }

// IsException returns true if the login is in the PRU exception list.
func (m *Manager) IsException(login string) bool {
	return m.exceptions[strings.ToLower(login)]
}

// AssignCostCenter returns the cost center ID for a given user.
//
//	exception user → pru_allowed_cost_center_id
//	everyone else  → no_prus_cost_center_id
func (m *Manager) AssignCostCenter(user github.CopilotUser) string {
	if m.IsException(user.Login) {
		m.log.Debug("User is PRU exception", "user", user.Login, "cc", m.pruAllowedCCID)
		return m.pruAllowedCCID
	}
	m.log.Debug("User assigned to default CC", "user", user.Login, "cc", m.noPRUCCID)
	return m.noPRUCCID
}

// AssignmentGroups builds the desired {cost_center_id: [usernames]} map for a
// list of users.
func (m *Manager) AssignmentGroups(users []github.CopilotUser) map[string][]string {
	groups := map[string][]string{
		m.pruAllowedCCID: {},
		m.noPRUCCID:      {},
	}
	for _, u := range users {
		cc := m.AssignCostCenter(u)
		groups[cc] = append(groups[cc], u.Login)
	}
	return groups
}

// GenerateSummary returns a cost-center → user-count map for display.
func (m *Manager) GenerateSummary(users []github.CopilotUser) map[string]int {
	summary := make(map[string]int)
	for _, u := range users {
		cc := m.AssignCostCenter(u)
		summary[cc]++
	}
	return summary
}

// ValidateConfiguration checks that the PRU configuration is usable and
// returns a list of issues (empty = valid).
func (m *Manager) ValidateConfiguration() []string {
	var issues []string
	if m.noPRUCCID == "" {
		issues = append(issues, "no_prus_cost_center_id is not defined")
	}
	if m.pruAllowedCCID == "" {
		issues = append(issues, "prus_allowed_cost_center_id is not defined")
	}
	if m.noPRUCCID != "" && m.noPRUCCID == m.pruAllowedCCID {
		issues = append(issues, "no_prus_cost_center_id and prus_allowed_cost_center_id cannot be the same")
	}
	return issues
}

// PrintConfigSummary displays the current PRU configuration to stdout.
func (m *Manager) PrintConfigSummary(cfg *config.Manager, autoCreate bool) {
	fmt.Println()
	fmt.Println("===== Current Configuration =====")
	fmt.Printf("Enterprise: %s\n", cfg.Enterprise)

	if autoCreate {
		fmt.Printf("No PRUs Cost Center: New cost center %q to be created\n", cfg.NoPRUsCostCenterName)
		fmt.Printf("PRUs Allowed Cost Center: New cost center %q to be created\n", cfg.PRUsAllowedCostCenterName)
	} else {
		fmt.Printf("No PRUs Cost Center: %s\n", m.noPRUCCID)
		printCCURL(cfg.Enterprise, m.noPRUCCID)

		fmt.Printf("PRUs Allowed Cost Center: %s\n", m.pruAllowedCCID)
		printCCURL(cfg.Enterprise, m.pruAllowedCCID)
	}

	fmt.Printf("PRUs Exception Users (%d):\n", len(cfg.PRUsExceptionUsers))
	for _, u := range cfg.PRUsExceptionUsers {
		fmt.Printf("  - %s\n", u)
	}
	fmt.Println("===== End of Configuration =====")
	fmt.Println()
}

// ShowSuccessSummary prints a comprehensive success summary at the end of a
// run, including cost center URLs, user statistics, and assignment results.
func ShowSuccessSummary(cfg *config.Manager, users []github.CopilotUser, originalCount *int, results map[string]map[string]bool, applied bool) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("SUCCESS SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	// Cost center links.
	if cfg.Enterprise != "" && !strings.HasPrefix(cfg.Enterprise, "REPLACE_WITH_") {
		fmt.Printf("\nCOST CENTERS (%s):\n", cfg.Enterprise)
		if !strings.HasPrefix(cfg.NoPRUsCostCenterID, "REPLACE_WITH_") {
			fmt.Printf("  No PRU Overages: %s\n", cfg.NoPRUsCostCenterID)
			fmt.Printf("     -> https://github.com/enterprises/%s/billing/cost_centers/%s\n",
				cfg.Enterprise, cfg.NoPRUsCostCenterID)
		}
		if !strings.HasPrefix(cfg.PRUsAllowedCostCenterID, "REPLACE_WITH_") {
			fmt.Printf("  PRU Overages Allowed: %s\n", cfg.PRUsAllowedCostCenterID)
			fmt.Printf("     -> https://github.com/enterprises/%s/billing/cost_centers/%s\n",
				cfg.Enterprise, cfg.PRUsAllowedCostCenterID)
		}
	}

	// User statistics.
	if len(users) > 0 {
		fmt.Printf("\nUSER STATISTICS:\n")
		fmt.Printf("  Total users processed: %d\n", len(users))
		if originalCount != nil {
			fmt.Printf("  Incremental processing: %d of %d total users\n", len(users), *originalCount)
		}

		if results != nil && applied {
			totalAttempted := 0
			totalSuccessful := 0
			for _, ccResults := range results {
				for _, ok := range ccResults {
					totalAttempted++
					if ok {
						totalSuccessful++
					}
				}
			}
			fmt.Printf("  Assignment success rate: %d/%d users\n", totalSuccessful, totalAttempted)
			if totalSuccessful < totalAttempted {
				fmt.Printf("  Failed assignments: %d users\n", totalAttempted-totalSuccessful)
			}
		}
	}

	fmt.Println(strings.Repeat("=", 60))
}

// printCCURL prints the cost center URL if the IDs are not placeholders.
func printCCURL(enterprise, ccID string) {
	if enterprise == "" || strings.HasPrefix(enterprise, "REPLACE_WITH_") {
		return
	}
	if strings.HasPrefix(ccID, "REPLACE_WITH_") {
		return
	}
	fmt.Printf("  -> https://github.com/enterprises/%s/billing/cost_centers/%s\n", enterprise, ccID)
}
