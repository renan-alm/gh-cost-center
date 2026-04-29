package customprop

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// testLogger returns a quiet logger for test usage.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestClientFromURL creates a github.Client pointing at the given httptest server URL.
func newTestClientFromURL(t *testing.T, url string) *github.Client {
	t.Helper()
	cfg := &config.Manager{
		Enterprise: "test-ent",
		APIBaseURL: url,
		Token:      "test-token",
	}
	c, err := github.NewClient(cfg, testLogger())
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	return c
}

// newTestManagerWithClient builds a Manager with a real client and budget products.
func newTestManagerWithClient(t *testing.T, client *github.Client, products map[string]config.ProductBudget) *Manager {
	t.Helper()
	cfg := &config.Manager{
		BudgetsEnabled: true,
		BudgetProducts: products,
	}
	return &Manager{
		cfg:    cfg,
		client: client,
		log:    testLogger(),
	}
}

func TestCreateBudgets_AllSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"budgets": []any{}})
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := newTestManagerWithClient(t, client, products)

	err := mgr.createBudgets("cc-id-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCreateBudgets_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"budgets": []any{}})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := newTestManagerWithClient(t, client, products)

	err := mgr.createBudgets("cc-id-1", "Fail CC")
	if err == nil {
		t.Fatal("expected error for budget creation failure")
	}
	if !strings.Contains(err.Error(), "budget creation failed for cost center Fail CC") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateBudgets_APIUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := newTestManagerWithClient(t, client, products)

	// 404 triggers BudgetsAPIUnavailableError — graceful degradation, returns nil.
	err := mgr.createBudgets("cc-id-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error for API unavailable, got %v", err)
	}
}

func TestCreateBudgets_DisabledProducts(t *testing.T) {
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: false},
	}
	mgr := &Manager{
		cfg: &config.Manager{BudgetProducts: products},
		log: testLogger(),
	}

	err := mgr.createBudgets("cc-id-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error when all products disabled, got %v", err)
	}
}

// --- ValidateFiltersAgainstSchema tests ---

func TestValidateFiltersAgainstSchema_AllValid(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "backend"},
			},
		},
	})

	schema := []config.RepoCustomPropertyDef{
		{Name: "team", ValueType: "single_select", AllowedValues: []string{"backend", "frontend"}},
	}

	warnings := mgr.ValidateFiltersAgainstSchema(schema)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestValidateFiltersAgainstSchema_UnknownProperty(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "unknown-prop", Value: "x"},
			},
		},
	})

	schema := []config.RepoCustomPropertyDef{
		{Name: "team", ValueType: "string"},
	}

	warnings := mgr.ValidateFiltersAgainstSchema(schema)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "not defined in repo_custom_properties schema") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestValidateFiltersAgainstSchema_InvalidValue(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "devops"},
			},
		},
	})

	schema := []config.RepoCustomPropertyDef{
		{Name: "team", ValueType: "single_select", AllowedValues: []string{"backend", "frontend"}},
	}

	warnings := mgr.ValidateFiltersAgainstSchema(schema)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "not in allowed_values") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestValidateFiltersAgainstSchema_EmptySchema(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name:    "Backend",
			Filters: []config.CustomPropertyFilter{{Property: "team", Value: "x"}},
		},
	})

	warnings := mgr.ValidateFiltersAgainstSchema(nil)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil schema, got %v", warnings)
	}
}

func TestValidateFiltersAgainstSchema_StringTypeNoAllowedValues(t *testing.T) {
	mgr := newTestManager([]config.CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []config.CustomPropertyFilter{
				{Property: "team", Value: "anything"},
			},
		},
	})

	schema := []config.RepoCustomPropertyDef{
		{Name: "team", ValueType: "string"},
	}

	warnings := mgr.ValidateFiltersAgainstSchema(schema)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for string type without allowed_values, got %v", warnings)
	}
}

// --- GenerateSummary tests ---

func TestGenerateSummary(t *testing.T) {
	// Mock HTTP server returning repos with properties.
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
		{
			RepositoryName:     "repo3",
			RepositoryFullName: "org/repo3",
			Properties: []github.Property{
				{PropertyName: "team", Value: "backend"},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(repos)
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	costCenters := []config.CustomPropCostCenter{
		{
			Name:    "Backend",
			Filters: []config.CustomPropertyFilter{{Property: "team", Value: "backend"}},
		},
		{
			Name:    "Frontend",
			Filters: []config.CustomPropertyFilter{{Property: "team", Value: "frontend"}},
		},
	}

	cfg := &config.Manager{
		CustomPropCostCenters: costCenters,
	}
	mgr, err := NewManager(cfg, client, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	summary, err := mgr.GenerateSummary("org")
	if err != nil {
		t.Fatalf("GenerateSummary error: %v", err)
	}

	if summary.TotalRepos != 3 {
		t.Errorf("TotalRepos = %d, want 3", summary.TotalRepos)
	}
	if summary.TotalCCs != 2 {
		t.Errorf("TotalCCs = %d, want 2", summary.TotalCCs)
	}
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summary.Results))
	}
	if summary.Results[0].ReposMatched != 2 {
		t.Errorf("Backend matched = %d, want 2", summary.Results[0].ReposMatched)
	}
	if summary.Results[1].ReposMatched != 1 {
		t.Errorf("Frontend matched = %d, want 1", summary.Results[1].ReposMatched)
	}
}

// --- removeUnmatchedRepos tests ---

func TestRemoveUnmatchedRepos_RemovesStale(t *testing.T) {
	const testCCID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	var removedRepos []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/cost-centers/"+testCCID) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    testCCID,
				"name":  "Backend",
				"state": "active",
				"resources": []map[string]string{
					{"type": "Repository", "name": "org/repo1"},
					{"type": "Repository", "name": "org/repo2"},
					{"type": "Repository", "name": "org/stale-repo"},
				},
			})
			return
		}

		if r.Method == http.MethodDelete {
			var body map[string][]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			removedRepos = body["repositories"]
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	mgr := &Manager{
		cfg:    &config.Manager{CustomPropRemoveUnmatched: true},
		client: client,
		log:    testLogger(),
	}

	removed, err := mgr.removeUnmatchedRepos(testCCID, "Backend", []string{"org/repo1", "org/repo2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(removedRepos) != 1 || removedRepos[0] != "org/stale-repo" {
		t.Errorf("unexpected removed repos: %v", removedRepos)
	}
}

func TestRemoveUnmatchedRepos_NothingToRemove(t *testing.T) {
	const testCCID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    testCCID,
			"name":  "Backend",
			"state": "active",
			"resources": []map[string]string{
				{"type": "Repository", "name": "org/repo1"},
			},
		})
	}))
	defer srv.Close()

	client := newTestClientFromURL(t, srv.URL)
	mgr := &Manager{
		cfg:    &config.Manager{CustomPropRemoveUnmatched: true},
		client: client,
		log:    testLogger(),
	}

	removed, err := mgr.removeUnmatchedRepos(testCCID, "Backend", []string{"org/repo1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}
