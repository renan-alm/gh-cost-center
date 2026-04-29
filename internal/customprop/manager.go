// Package customprop implements custom-property-based cost center assignment
// for GitHub Enterprise repositories.  Each cost center specifies a set of
// filters that are combined with AND logic — a repository must satisfy every
// filter to be included in that cost center.
package customprop

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/renan-alm/gh-cost-center/internal/config"
	"github.com/renan-alm/gh-cost-center/internal/github"
)

// Result records the outcome of processing a single custom-property cost center.
type Result struct {
	CostCenter    string
	CostCenterID  string
	Filters       []config.CustomPropertyFilter
	ReposMatched  int
	ReposAssigned int
	ReposSkipped  int
	ReposRemoved  int
	Success       bool
	Message       string
}

// Summary holds the overall result of a custom-property assignment run.
type Summary struct {
	TotalRepos int
	TotalCCs   int
	AppliedCCs int
	Results    []Result
}

// Print displays the summary to stdout.
func (s *Summary) Print() {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("CUSTOM-PROPERTY ASSIGNMENT SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total repositories in organization: %d\n", s.TotalRepos)
	fmt.Printf("Cost centers processed: %d / %d\n", s.AppliedCCs, s.TotalCCs)

	for _, r := range s.Results {
		fmt.Println()
		fmt.Printf("Cost Center: %s\n", r.CostCenter)
		fmt.Println("  Filters (AND):")
		for _, f := range r.Filters {
			fmt.Printf("    %s = %q\n", f.Property, f.Value)
		}
		fmt.Printf("  Matched:   %d repositories\n", r.ReposMatched)
		fmt.Printf("  Assigned:  %d repositories\n", r.ReposAssigned)
		if r.ReposSkipped > 0 {
			fmt.Printf("  Skipped:   %d repositories (missing full name)\n", r.ReposSkipped)
		}
		if r.ReposRemoved > 0 {
			fmt.Printf("  Removed:   %d repositories (no longer match filters)\n", r.ReposRemoved)
		}
		if r.Success {
			fmt.Println("  Status:    Success")
		} else {
			fmt.Printf("  Status:    Failed — %s\n", r.Message)
		}
	}
	fmt.Println(strings.Repeat("=", 80))
}

// Manager discovers repositories using GitHub custom property filters and
// assigns them to cost centers.
type Manager struct {
	cfg         *config.Manager
	client      *github.Client
	log         *slog.Logger
	costCenters []config.CustomPropCostCenter
}

// NewManager creates a Manager from configuration.
// It returns an error if no custom-property cost centers are configured.
func NewManager(cfg *config.Manager, client *github.Client, logger *slog.Logger) (*Manager, error) {
	if len(cfg.CustomPropCostCenters) == 0 {
		return nil, fmt.Errorf("custom-prop mode requires at least one entry in cost_center.custom_prop.cost_centers")
	}
	return &Manager{
		cfg:         cfg,
		client:      client,
		log:         logger,
		costCenters: cfg.CustomPropCostCenters,
	}, nil
}

// ValidateConfiguration checks the custom-property cost center definitions and
// returns a list of human-readable issues found.
func (m *Manager) ValidateConfiguration() []string {
	var issues []string
	seen := make(map[string]bool, len(m.costCenters))
	for i, cc := range m.costCenters {
		if cc.Name == "" {
			issues = append(issues, fmt.Sprintf("cost center %d: missing name", i+1))
		}
		if len(cc.Filters) == 0 {
			issues = append(issues, fmt.Sprintf("cost center %d (%q): no filters defined", i+1, cc.Name))
		}
		for j, f := range cc.Filters {
			if f.Property == "" {
				issues = append(issues, fmt.Sprintf("cost center %d (%q) filter %d: missing property", i+1, cc.Name, j+1))
			}
			if f.Value == "" {
				issues = append(issues, fmt.Sprintf("cost center %d (%q) filter %d: missing value", i+1, cc.Name, j+1))
			}
		}
		if cc.Name != "" && seen[cc.Name] {
			issues = append(issues, fmt.Sprintf("duplicate cost center name %q", cc.Name))
		}
		seen[cc.Name] = true
	}
	return issues
}

// ValidateFiltersAgainstSchema checks that filter property names and values
// are consistent with the repo custom property schema definitions.  Returns
// human-readable warnings (not errors) because the schema may be incomplete.
func (m *Manager) ValidateFiltersAgainstSchema(schema []config.RepoCustomPropertyDef) []string {
	if len(schema) == 0 {
		return nil
	}

	// Build lookup maps from schema.
	schemaDefs := make(map[string]config.RepoCustomPropertyDef, len(schema))
	for _, d := range schema {
		schemaDefs[d.Name] = d
	}

	var warnings []string
	for _, cc := range m.costCenters {
		for _, f := range cc.Filters {
			def, exists := schemaDefs[f.Property]
			if !exists {
				warnings = append(warnings, fmt.Sprintf(
					"cost center %q: filter property %q is not defined in repo_custom_properties schema",
					cc.Name, f.Property))
				continue
			}
			if len(def.AllowedValues) > 0 {
				found := false
				for _, v := range def.AllowedValues {
					if v == f.Value {
						found = true
						break
					}
				}
				if !found {
					warnings = append(warnings, fmt.Sprintf(
						"cost center %q: filter value %q for property %q is not in allowed_values %v",
						cc.Name, f.Value, f.Property, def.AllowedValues))
				}
			}
		}
	}
	return warnings
}

// PrintConfigSummary displays the custom-property configuration.
func (m *Manager) PrintConfigSummary(org string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Custom-Property Cost Center Assignment")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Organization:  %s\n", org)
	fmt.Printf("Cost Centers:  %d\n", len(m.costCenters))
	for i, cc := range m.costCenters {
		fmt.Printf("\n  Cost Center %d: %s\n", i+1, cc.Name)
		fmt.Println("    Filters (AND logic — all must match):")
		for _, f := range cc.Filters {
			fmt.Printf("      %s = %q\n", f.Property, f.Value)
		}
	}
	fmt.Println(strings.Repeat("=", 80))
}

// GenerateSummary produces a read-only summary of which repositories match
// each custom-property cost center.  It does NOT create or assign anything.
func (m *Manager) GenerateSummary(org string) (*Summary, error) {
	m.log.Info("Generating custom-property cost center summary", "org", org)

	allRepos, err := m.client.GetOrgReposWithProperties(org, "")
	if err != nil {
		return nil, fmt.Errorf("fetching repos with properties: %w", err)
	}

	summary := &Summary{
		TotalRepos: len(allRepos),
		TotalCCs:   len(m.costCenters),
	}

	for _, cc := range m.costCenters {
		matching := findReposMatchingAllFilters(allRepos, cc.Filters)
		result := Result{
			CostCenter:    cc.Name,
			Filters:       cc.Filters,
			ReposMatched:  len(matching),
			ReposAssigned: len(matching),
			Success:       true,
			Message:       fmt.Sprintf("%d repositories match", len(matching)),
		}
		if len(matching) == 0 {
			result.Message = "no repositories matched all filters"
		}
		summary.AppliedCCs++
		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}

// Run executes the full custom-property assignment flow.
// mode is "plan" or "apply".  createBudgets enables budget creation for new CCs.
func (m *Manager) Run(org, mode string, createBudgets bool) (*Summary, error) {
	m.log.Info("Starting custom-property cost center assignment",
		"org", org, "mode", mode, "cost_centers", len(m.costCenters))

	// Fetch all repos with custom properties.
	m.log.Info("Fetching repositories with custom properties...", "org", org)
	allRepos, err := m.client.GetOrgReposWithProperties(org, "")
	if err != nil {
		return nil, fmt.Errorf("fetching repos with properties: %w", err)
	}
	if len(allRepos) == 0 {
		m.log.Warn("No repositories found", "org", org)
		return &Summary{TotalRepos: 0, TotalCCs: len(m.costCenters)}, nil
	}
	m.log.Info("Repositories found", "org", org, "count", len(allRepos))

	// Preload existing cost centers for efficient lookups.
	activeCCs, err := m.client.GetAllActiveCostCenters()
	if err != nil {
		return nil, fmt.Errorf("fetching active cost centers: %w", err)
	}
	m.log.Info("Existing cost centers loaded", "count", len(activeCCs))

	summary := &Summary{
		TotalRepos: len(allRepos),
		TotalCCs:   len(m.costCenters),
	}

	// Process each custom-property cost center.
	for i, cc := range m.costCenters {
		m.log.Info("Processing cost center",
			"index", i+1, "total", len(m.costCenters),
			"name", cc.Name, "filters", len(cc.Filters))

		result := m.processCostCenter(cc, allRepos, activeCCs, mode, createBudgets)
		if result.Success {
			summary.AppliedCCs++
		}
		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}

// processCostCenter handles one custom-property cost center — finds matching
// repos and (in apply mode) ensures the CC exists and assigns the repos.
func (m *Manager) processCostCenter(
	cc config.CustomPropCostCenter,
	allRepos []github.RepoProperties,
	activeCCs map[string]string,
	mode string,
	createBudgets bool,
) Result {
	result := Result{
		CostCenter: cc.Name,
		Filters:    cc.Filters,
	}

	// Find repos that satisfy all filters (AND logic).
	matching := findReposMatchingAllFilters(allRepos, cc.Filters)
	result.ReposMatched = len(matching)

	if len(matching) == 0 {
		result.Message = "no repositories matched all filters"
		m.log.Warn("No repos matched all filters",
			"cost_center", cc.Name, "filters", len(cc.Filters))
		return result
	}

	m.log.Info("Repositories matched",
		"cost_center", cc.Name, "count", len(matching))

	// Plan mode — report what would happen without making changes.
	if mode == "plan" {
		result.ReposAssigned = len(matching)
		result.Success = true
		result.Message = fmt.Sprintf("would assign %d repositories (plan mode)", len(matching))
		m.log.Info("mode=plan: would assign repos",
			"cost_center", cc.Name, "count", len(matching))
		for _, r := range matching {
			m.log.Debug("Would assign", "repo", r.RepositoryFullName, "cost_center", cc.Name)
		}
		return result
	}

	// Apply mode — ensure the cost center exists.
	ccID, ok := activeCCs[cc.Name]
	if !ok {
		m.log.Info("Cost center does not exist, creating...", "name", cc.Name)
		var err error
		ccID, err = m.client.CreateCostCenterWithPreload(cc.Name, activeCCs)
		if err != nil {
			result.Message = fmt.Sprintf("failed to create cost center: %v", err)
			m.log.Error("Failed to create cost center", "name", cc.Name, "error", err)
			return result
		}
		activeCCs[cc.Name] = ccID
		m.log.Info("Created cost center", "name", cc.Name, "id", ccID)

		if createBudgets && m.cfg.BudgetsEnabled {
			if err := m.createBudgets(ccID, cc.Name); err != nil {
				result.Message = fmt.Sprintf("budget creation failed: %v", err)
				m.log.Error("Budget creation failed for cost center", "name", cc.Name, "error", err)
				return result
			}
		}
	} else {
		m.log.Info("Cost center already exists", "name", cc.Name, "id", ccID)
	}

	result.CostCenterID = ccID

	// Collect repo full names.
	repoNames := make([]string, 0, len(matching))
	for _, r := range matching {
		if r.RepositoryFullName != "" {
			repoNames = append(repoNames, r.RepositoryFullName)
		} else {
			result.ReposSkipped++
			m.log.Warn("Repository missing full name, skipping", "name", r.RepositoryName)
		}
	}

	if len(repoNames) == 0 {
		result.Message = "no valid repository names to assign"
		m.log.Error("No valid repo names", "cost_center", cc.Name)
		return result
	}

	for i, name := range repoNames {
		if i < 10 {
			m.log.Info("Assigning repo", "repo", name, "cost_center", cc.Name)
		}
	}
	if len(repoNames) > 10 {
		m.log.Info("...and more", "remaining", len(repoNames)-10)
	}

	if err := m.client.AddRepositoriesToCostCenter(ccID, repoNames); err != nil {
		result.Message = fmt.Sprintf("failed to assign repos: %v", err)
		m.log.Error("Failed to assign repos", "cost_center", cc.Name, "error", err)
		return result
	}

	result.ReposAssigned = len(repoNames)
	result.Success = true
	result.Message = fmt.Sprintf("successfully assigned %d/%d repositories",
		len(repoNames), len(matching))
	m.log.Info("Successfully assigned repos",
		"cost_center", cc.Name, "assigned", len(repoNames))

	// Remove repos that no longer match filters (if enabled).
	if m.cfg.CustomPropRemoveUnmatched {
		removed, err := m.removeUnmatchedRepos(ccID, cc.Name, repoNames)
		if err != nil {
			m.log.Error("Failed to remove unmatched repos", "cost_center", cc.Name, "error", err)
		} else {
			result.ReposRemoved = removed
		}
	}

	return result
}

// removeUnmatchedRepos removes repositories that are currently in the cost
// center but no longer match the filters.
func (m *Manager) removeUnmatchedRepos(ccID, ccName string, matchedRepos []string) (int, error) {
	currentRepos, err := m.client.GetCostCenterRepos(ccID)
	if err != nil {
		return 0, fmt.Errorf("fetching current repos for %s: %w", ccName, err)
	}

	matchedSet := make(map[string]bool, len(matchedRepos))
	for _, r := range matchedRepos {
		matchedSet[r] = true
	}

	var stale []string
	for _, r := range currentRepos {
		if !matchedSet[r] {
			stale = append(stale, r)
		}
	}

	if len(stale) == 0 {
		m.log.Info("No unmatched repos to remove", "cost_center", ccName)
		return 0, nil
	}

	m.log.Info("Removing unmatched repos from cost center",
		"cost_center", ccName, "count", len(stale))
	for _, r := range stale {
		m.log.Debug("Removing repo", "repo", r, "cost_center", ccName)
	}

	if err := m.client.RemoveRepositoriesFromCostCenter(ccID, stale); err != nil {
		return 0, fmt.Errorf("removing unmatched repos from %s: %w", ccName, err)
	}

	return len(stale), nil
}

// createBudgets creates configured budgets for a newly-created cost center.
func (m *Manager) createBudgets(ccID, ccName string) error {
	m.log.Info("Creating budgets for cost center", "name", ccName)

	var failures []string
	for product, pc := range m.cfg.BudgetProducts {
		if !pc.Enabled {
			m.log.Debug("Skipping disabled product budget", "product", product)
			continue
		}

		ok, err := m.client.CreateProductBudget(ccID, ccName, product, pc.Amount)
		if err != nil {
			if _, unavailable := err.(*github.BudgetsAPIUnavailableError); unavailable {
				m.log.Warn("Budgets API unavailable, skipping remaining budgets", "error", err)
				return nil
			}
			m.log.Error("Failed to create budget",
				"product", product, "cost_center", ccName, "error", err)
			failures = append(failures, fmt.Sprintf("%s: %v", product, err))
			continue
		}
		if ok {
			m.log.Info("Budget created",
				"product", product, "cost_center", ccName, "amount", pc.Amount)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("budget creation failed for cost center %s: %s", ccName, strings.Join(failures, "; "))
	}
	return nil
}

// findReposMatchingAllFilters returns the repos that satisfy every filter
// (AND logic).
func findReposMatchingAllFilters(
	repos []github.RepoProperties,
	filters []config.CustomPropertyFilter,
) []github.RepoProperties {
	if len(filters) == 0 {
		return nil
	}

	var matched []github.RepoProperties
	for _, repo := range repos {
		if repoMatchesAllFilters(repo, filters) {
			matched = append(matched, repo)
		}
	}
	return matched
}

// repoMatchesAllFilters returns true when the repository satisfies every
// filter (AND logic).
func repoMatchesAllFilters(repo github.RepoProperties, filters []config.CustomPropertyFilter) bool {
	propMap := make(map[string]any, len(repo.Properties))
	for _, p := range repo.Properties {
		propMap[p.PropertyName] = p.Value
	}

	for _, f := range filters {
		val, exists := propMap[f.Property]
		if !exists {
			return false
		}
		if !matchesValue(val, f.Value) {
			return false
		}
	}
	return true
}

// matchesValue checks if a property value (string or []any) matches the
// expected value.
func matchesValue(val any, expected string) bool {
	switch v := val.(type) {
	case string:
		return v == expected
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}
