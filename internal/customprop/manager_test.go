package customprop

import (
	"log/slog"
	"os"
	"testing"

	"github.com/renan-alm/gh-cost-center/internal/config"
	"github.com/renan-alm/gh-cost-center/internal/github"
)

// newTestManager creates a Manager with test defaults.
func newTestManager(costCenters []config.CustomPropCostCenter) *Manager {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Manager{
		CustomPropCostCenters: costCenters,
		BudgetsEnabled:        false,
	}
	return &Manager{
		cfg:         cfg,
		log:         logger,
		costCenters: costCenters,
	}
}

// --- NewManager tests ---

func TestNewManager_NoCostCenters(t *testing.T) {
	cfg := &config.Manager{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	_, err := NewManager(cfg, nil, logger)
	if err == nil {
		t.Fatal("expected error for empty CustomPropCostCenters")
	}
}

func TestNewManager_Valid(t *testing.T) {
	cfg := &config.Manager{
		CustomPropCostCenters: []config.CustomPropCostCenter{
			{
				Name: "Backend",
				Filters: []config.CustomPropertyFilter{
					{Property: "team", Value: "backend"},
				},
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mgr, err := NewManager(cfg, nil, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mgr.costCenters) != 1 {
		t.Errorf("expected 1 cost center, got %d", len(mgr.costCenters))
	}
}

// --- ValidateConfiguration tests ---

func TestValidateConfiguration_AllValid(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend"},
				{Property: "env", Value: "prod"},
			},
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestValidateConfiguration_MissingName(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend"},
			},
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) == 0 {
		t.Error("expected issue for missing name")
	}
}

func TestValidateConfiguration_NoFilters(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name:    "Backend",
			Filters: nil,
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) == 0 {
		t.Error("expected issue for no filters")
	}
}

func TestValidateConfiguration_MissingFilterProperty(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "", Value: "backend"},
			},
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) == 0 {
		t.Error("expected issue for missing filter property")
	}
}

func TestValidateConfiguration_MissingFilterValue(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: ""},
			},
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) == 0 {
		t.Error("expected issue for missing filter value")
	}
}

func TestValidateConfiguration_DuplicateName(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend"},
			},
		},
		{
			Name: "Backend", // duplicate
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend-v2"},
			},
		},
	})

	issues := mgr.ValidateConfiguration()
	if len(issues) == 0 {
		t.Error("expected issue for duplicate name")
	}
}

// --- findReposMatchingAllFilters tests ---

func TestFindReposMatchingAllFilters_SingleFilter(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
			},
		},
		{
			RepositoryName:     "repo2",
			RepositoryFullName: "org/repo2",
			Properties: []github.Property{
				{PropertyName: "team", Value: "frontend"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 1 {
		t.Errorf("expected 1 match, got %d", len(matched))
	}
	if matched[0].RepositoryName != "repo1" {
		t.Errorf("expected repo1, got %s", matched[0].RepositoryName)
	}
}

func TestFindReposMatchingAllFilters_ANDLogic(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
				{PropertyName: "env", Value: "prod"},
			},
		},
		{
			RepositoryName:     "repo2",
			RepositoryFullName: "org/repo2",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
				{PropertyName: "env", Value: "staging"},
			},
		},
		{
			RepositoryName:     "repo3",
			RepositoryFullName: "org/repo3",
			Properties: []github.Property{
				{PropertyName: "team", Value: "frontend"},
				{PropertyName: "env", Value: "prod"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
		{Property: "env", Value: "prod"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 1 {
		t.Errorf("expected 1 match (AND logic), got %d", len(matched))
	}
	if matched[0].RepositoryName != "repo1" {
		t.Errorf("expected repo1, got %s", matched[0].RepositoryName)
	}
}

func TestFindReposMatchingAllFilters_ThreeFilters(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
				{PropertyName: "env", Value: "prod"},
				{PropertyName: "cost-center-id", Value: "CC-1234"},
			},
		},
		{
			RepositoryName:     "repo2",
			RepositoryFullName: "org/repo2",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
				{PropertyName: "env", Value: "prod"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
		{Property: "env", Value: "prod"},
		{Property: "cost-center-id", Value: "CC-1234"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 1 {
		t.Errorf("expected 1 match, got %d", len(matched))
	}
	if matched[0].RepositoryName != "repo1" {
		t.Errorf("expected repo1, got %s", matched[0].RepositoryName)
	}
}

func TestFindReposMatchingAllFilters_NoMatch(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "frontend"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matched))
	}
}

func TestFindReposMatchingAllFilters_EmptyFilters(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties:         []github.Property{{PropertyName: "team", Value: "backend"}},
		},
	}

	matched := findReposMatchingAllFilters(repos, nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 matches for empty filters, got %d", len(matched))
	}
}

func TestFindReposMatchingAllFilters_EmptyRepos(t *testing.T) {
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
	}

	matched := findReposMatchingAllFilters(nil, filters)
	if len(matched) != 0 {
		t.Errorf("expected 0 matches for empty repos, got %d", len(matched))
	}
}

func TestFindReposMatchingAllFilters_MissingProperty(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
		{Property: "env", Value: "prod"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 0 {
		t.Errorf("expected 0 matches when a required property is missing, got %d", len(matched))
	}
}

func TestFindReposMatchingAllFilters_ArrayValueMatch(t *testing.T) {
	repos := []github.RepoProperties{
		{
			RepositoryName:     "repo1",
			RepositoryFullName: "org/repo1",
			Properties: []github.Property{
				{PropertyName: "tags", Value: []any{"go", "backend", "api"}},
				{PropertyName: "env", Value: "prod"},
			},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "tags", Value: "backend"},
		{Property: "env", Value: "prod"},
	}

	matched := findReposMatchingAllFilters(repos, filters)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for array property, got %d", len(matched))
	}
}

// --- repoMatchesAllFilters tests ---

func TestRepoMatchesAllFilters_AllMatch(t *testing.T) {
	repo := github.RepoProperties{
		Properties: []github.Property{
			{PropertyName: "team", Value: "backend"},
			{PropertyName: "env", Value: "prod"},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
		{Property: "env", Value: "prod"},
	}

	if !repoMatchesAllFilters(repo, filters) {
		t.Error("expected all filters to match")
	}
}

func TestRepoMatchesAllFilters_PartialMatch(t *testing.T) {
	repo := github.RepoProperties{
		Properties: []github.Property{
			{PropertyName: "team", Value: "backend"},
		},
	}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
		{Property: "env", Value: "prod"},
	}

	if repoMatchesAllFilters(repo, filters) {
		t.Error("expected false when a filter is not satisfied")
	}
}

func TestRepoMatchesAllFilters_NoProperties(t *testing.T) {
	repo := github.RepoProperties{Properties: nil}
	filters := []config.CustomPropertyFilter{
		{Property: "team", Value: "backend"},
	}

	if repoMatchesAllFilters(repo, filters) {
		t.Error("expected false for repo with no properties")
	}
}

// --- Summary.Print test ---

func TestSummaryPrint(t *testing.T) {
	s := &Summary{
		TotalRepos: 20,
		TotalCCs:   2,
		AppliedCCs: 1,
		Results: []Result{
			{
				CostCenter: "Backend",
				Filters: []config.CustomPropertyFilter{
					{Property: "team", Value: "backend"},
				},
				ReposMatched:  5,
				ReposAssigned: 5,
				Success:       true,
				Message:       "ok",
			},
			{
				CostCenter: "Frontend",
				Filters: []config.CustomPropertyFilter{
					{Property: "team", Value: "frontend"},
				},
				ReposMatched:  0,
				ReposAssigned: 0,
				Success:       false,
				Message:       "no repos matched",
			},
		},
	}
	// Verify it does not panic.
	s.Print()
}

// --- PrintConfigSummary test ---

func TestPrintConfigSummary(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend"},
				{Property: "cost-center-id", Value: "CC-1234"},
			},
		},
	})

	// Verify it does not panic.
	mgr.PrintConfigSummary("test-org")
}

// --- matchesValue tests ---

func TestMatchesValue_String(t *testing.T) {
	if !matchesValue("backend", "backend") {
		t.Error("expected string match")
	}
	if matchesValue("frontend", "backend") {
		t.Error("expected no match for different string")
	}
}

func TestMatchesValue_Array(t *testing.T) {
	arr := []any{"go", "backend", "api"}
	if !matchesValue(arr, "backend") {
		t.Error("expected array element match")
	}
	if matchesValue(arr, "frontend") {
		t.Error("expected no match for missing element")
	}
}

func TestMatchesValue_OtherType(t *testing.T) {
	if matchesValue(42, "42") {
		t.Error("expected no match for non-string/non-array type")
	}
}
