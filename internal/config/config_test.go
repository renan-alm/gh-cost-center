package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper to write a temp YAML config and return its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return p
}

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// ---------- Loading the example config ----------

func TestLoad_ExampleConfig(t *testing.T) {
	t.Setenv("GITHUB_ENTERPRISE", "test-enterprise")
	examplePath := filepath.Join("..", "..", "config", "config.example.yaml")
	m, err := Load(examplePath, logger())
	if err != nil {
		t.Fatalf("Load config.example.yaml: %v", err)
	}
	if m.Enterprise != "test-enterprise" {
		t.Errorf("enterprise = %q, want %q", m.Enterprise, "test-enterprise")
	}
	if m.CostCenterMode != "users" {
		t.Errorf("cost_center_mode = %q, want %q", m.CostCenterMode, "users")
	}
}

// ---------- Minimal valid config ----------

func TestLoad_MinimalConfig(t *testing.T) {
	yaml := `
github:
  enterprise: "my-ent"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "my-ent" {
		t.Errorf("enterprise = %q", m.Enterprise)
	}
	if m.APIBaseURL != DefaultAPIBaseURL {
		t.Errorf("api_base_url = %q, want default", m.APIBaseURL)
	}
	if m.CostCenterMode != DefaultCostCenterMode {
		t.Errorf("mode = %q, want %q", m.CostCenterMode, DefaultCostCenterMode)
	}
}

// ---------- Missing enterprise ----------

func TestLoad_MissingEnterprise(t *testing.T) {
	yaml := `
github:
  enterprise: ""
`
	t.Setenv("GITHUB_ENTERPRISE", "")
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for missing enterprise")
	}
}

// ---------- Enterprise placeholder in YAML, real value in env ----------

func TestLoad_EnterprisePlaceholderWithEnvOverride(t *testing.T) {
	yaml := `
github:
  enterprise: "your_enterprise_name"
`
	t.Setenv("GITHUB_ENTERPRISE", "real-ent")
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "real-ent" {
		t.Errorf("enterprise = %q, want %q", m.Enterprise, "real-ent")
	}
}

// ---------- Env var overrides ----------

func TestLoad_EnvVarOverrides(t *testing.T) {
	yaml := `
github:
  enterprise: "yaml-ent"
  api_base_url: "https://api.github.com"
`
	t.Setenv("GITHUB_ENTERPRISE", "env-ent")
	t.Setenv("GITHUB_API_BASE_URL", "https://api.corp.ghe.com")
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "env-ent" {
		t.Errorf("enterprise = %q, want env-ent", m.Enterprise)
	}
	if m.APIBaseURL != "https://api.corp.ghe.com" {
		t.Errorf("api_base_url = %q, want env value", m.APIBaseURL)
	}
}

func TestLoad_DotEnvLoadsWhenEnvMissing(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("GITHUB_ENTERPRISE=dotenv-ent\n"), 0o644); err != nil {
		t.Fatalf("writing .env: %v", err)
	}

	yaml := `
github:
  enterprise: ""
`
	if err := os.Unsetenv("GITHUB_ENTERPRISE"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "dotenv-ent" {
		t.Errorf("enterprise = %q, want %q", m.Enterprise, "dotenv-ent")
	}
}

func TestLoad_ExistingEnvBeatsDotEnv(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("GITHUB_ENTERPRISE=dotenv-ent\n"), 0o644); err != nil {
		t.Fatalf("writing .env: %v", err)
	}

	yaml := `
github:
  enterprise: "yaml-ent"
`
	t.Setenv("GITHUB_ENTERPRISE", "session-ent")
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "session-ent" {
		t.Errorf("enterprise = %q, want %q", m.Enterprise, "session-ent")
	}
}

// ---------- Users (PRU) mode ----------

func TestLoad_UsersMode(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "users"
  users:
    no_prus_cost_center_id: "CC-001"
    prus_allowed_cost_center_id: "CC-002"
    no_prus_cost_center_name: "No PRU"
    prus_allowed_cost_center_name: "PRU Allowed"
    exception_users:
      - "alice"
    auto_create: true
    enable_incremental: true
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CostCenterMode != "users" {
		t.Errorf("mode = %q", m.CostCenterMode)
	}
	if m.NoPRUsCostCenterID != "CC-001" {
		t.Errorf("NoPRUsCostCenterID = %q", m.NoPRUsCostCenterID)
	}
	if m.PRUsAllowedCostCenterID != "CC-002" {
		t.Errorf("PRUsAllowedCostCenterID = %q", m.PRUsAllowedCostCenterID)
	}
	if m.NoPRUsCostCenterName != "No PRU" {
		t.Errorf("NoPRUsCostCenterName = %q", m.NoPRUsCostCenterName)
	}
	if m.PRUsAllowedCostCenterName != "PRU Allowed" {
		t.Errorf("PRUsAllowedCostCenterName = %q", m.PRUsAllowedCostCenterName)
	}
	if len(m.PRUsExceptionUsers) != 1 || m.PRUsExceptionUsers[0] != "alice" {
		t.Errorf("PRUsExceptionUsers = %v", m.PRUsExceptionUsers)
	}
	if !m.AutoCreate {
		t.Error("expected AutoCreate = true")
	}
	if !m.EnableIncremental {
		t.Error("expected EnableIncremental = true")
	}
}

func TestLoad_UsersModeDefaults(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CostCenterMode != "users" {
		t.Errorf("mode = %q, want 'users'", m.CostCenterMode)
	}
	if m.NoPRUsCostCenterID != DefaultNoPRUsCCID {
		t.Errorf("NoPRUsCostCenterID = %q, want default", m.NoPRUsCostCenterID)
	}
	if m.PRUsAllowedCostCenterID != DefaultPRUsAllowedCCID {
		t.Errorf("PRUsAllowedCostCenterID = %q, want default", m.PRUsAllowedCostCenterID)
	}
}

// ---------- Teams mode ----------

func TestLoad_TeamsMode(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations:
    - "my-org"
cost_center:
  mode: "teams"
  teams:
    scope: "organization"
    strategy: "manual"
    auto_create: true
    remove_unmatched_users: true
    mappings:
      "my-org/frontend": "CC-FRONTEND"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CostCenterMode != "teams" {
		t.Errorf("mode = %q", m.CostCenterMode)
	}
	if m.TeamsScope != "organization" {
		t.Errorf("TeamsScope = %q", m.TeamsScope)
	}
	if m.TeamsStrategy != "manual" {
		t.Errorf("TeamsStrategy = %q", m.TeamsStrategy)
	}
	if !m.TeamsAutoCreate {
		t.Error("expected TeamsAutoCreate = true")
	}
	if !m.TeamsRemoveUnmatchedUsers {
		t.Error("expected TeamsRemoveUnmatchedUsers = true")
	}
	if m.TeamsMappings["my-org/frontend"] != "CC-FRONTEND" {
		t.Errorf("TeamsMappings = %v", m.TeamsMappings)
	}
}

func TestLoad_TeamsModeDefaults(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "teams"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.TeamsScope != DefaultTeamsScope {
		t.Errorf("TeamsScope = %q, want default %q", m.TeamsScope, DefaultTeamsScope)
	}
	if m.TeamsStrategy != DefaultTeamsStrategy {
		t.Errorf("TeamsStrategy = %q, want default %q", m.TeamsStrategy, DefaultTeamsStrategy)
	}
}

func TestLoad_TeamsModeOrgScopeRequiresOrgs(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "teams"
  teams:
    scope: "organization"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for organization scope without organizations")
	}
}

func TestLoad_TeamsModeInvalidStrategy(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "teams"
  teams:
    strategy: "badvalue"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}

func TestLooksLikeUUID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"d1e2f3a4-b5c6-7890-abcd-ef1234567890", true},
		{"D1E2F3A4-B5C6-7890-ABCD-EF1234567890", true},
		{"42_Ölbrück-Straße", false},
		{"my-cost-center", false},
		{"[org team] my-org/devs", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeUUID(tt.input); got != tt.want {
				t.Errorf("looksLikeUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- Repos mode ----------

func TestLoad_ReposMode(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations:
    - "my-org"
cost_center:
  mode: "repos"
  repos:
    mappings:
      - cost_center: "Platform"
        property_name: "team"
        property_values:
          - "platform"
          - "infra"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CostCenterMode != "repos" {
		t.Errorf("mode = %q", m.CostCenterMode)
	}
	if len(m.ReposMappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(m.ReposMappings))
	}
	if m.ReposMappings[0].CostCenter != "Platform" {
		t.Error("wrong cost center")
	}
	if len(m.ReposMappings[0].PropertyValues) != 2 {
		t.Errorf("expected 2 property values, got %d", len(m.ReposMappings[0].PropertyValues))
	}
}

func TestLoad_ReposModeRequiresOrgs(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "repos"
  repos:
    mappings:
      - cost_center: "CC"
        property_name: "team"
        property_values: ["x"]
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for repos mode without organizations")
	}
}

func TestLoad_ReposModeRequiresMappings(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations: ["org"]
cost_center:
  mode: "repos"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for repos mode without mappings")
	}
}

// ---------- Custom-prop mode ----------

func TestLoad_CustomPropMode(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations:
    - "my-org"
cost_center:
  mode: "custom-prop"
  custom_prop:
    cost_centers:
      - name: "Backend Engineering"
        filters:
          - property: "team"
            value: "backend"
          - property: "cost-center-id"
            value: "CC-1234"
      - name: "Frontend Engineering"
        filters:
          - property: "team"
            value: "frontend"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.CostCenterMode != "custom-prop" {
		t.Errorf("mode = %q", m.CostCenterMode)
	}
	if len(m.CustomPropCostCenters) != 2 {
		t.Fatalf("expected 2 custom-prop cost centers, got %d", len(m.CustomPropCostCenters))
	}

	backend := m.CustomPropCostCenters[0]
	if backend.Name != "Backend Engineering" {
		t.Errorf("name = %q", backend.Name)
	}
	if len(backend.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(backend.Filters))
	}
	if backend.Filters[0].Property != "team" || backend.Filters[0].Value != "backend" {
		t.Errorf("filter[0] = {%q, %q}", backend.Filters[0].Property, backend.Filters[0].Value)
	}
	if backend.Filters[1].Property != "cost-center-id" || backend.Filters[1].Value != "CC-1234" {
		t.Errorf("filter[1] = {%q, %q}", backend.Filters[1].Property, backend.Filters[1].Value)
	}
}

func TestLoad_CustomPropModeRequiresOrgs(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "custom-prop"
  custom_prop:
    cost_centers:
      - name: "Backend"
        filters:
          - property: "team"
            value: "backend"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for custom-prop mode without organizations")
	}
}

func TestLoad_CustomPropModeMissingName(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations: ["org"]
cost_center:
  mode: "custom-prop"
  custom_prop:
    cost_centers:
      - name: ""
        filters:
          - property: "team"
            value: "backend"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_CustomPropModeNoFilters(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations: ["org"]
cost_center:
  mode: "custom-prop"
  custom_prop:
    cost_centers:
      - name: "Backend"
        filters: []
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for empty filters")
	}
}

func TestLoad_CustomPropModeDuplicateName(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
  organizations: ["org"]
cost_center:
  mode: "custom-prop"
  custom_prop:
    cost_centers:
      - name: "Backend"
        filters:
          - property: "team"
            value: "backend"
      - name: "Backend"
        filters:
          - property: "env"
            value: "prod"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

// ---------- Invalid mode ----------

func TestLoad_InvalidMode(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "invalid"
`
	_, err := Load(writeConfig(t, yaml), logger())
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

// ---------- API URL validation ----------

func TestValidateAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		want    string
	}{
		{"standard github", "https://api.github.com", false, "https://api.github.com"},
		{"standard with trailing slash", "https://api.github.com/", false, "https://api.github.com"},
		{"ghe data resident", "https://api.corp.ghe.com", false, "https://api.corp.ghe.com"},
		{"ghe server", "https://github.myco.com/api/v3", false, "https://github.myco.com/api/v3"},
		{"http rejected", "http://api.github.com", true, ""},
		{"empty string", "", true, ""},
		{"bad ghe pattern", "https://corp.ghe.com", true, ""},
		{"custom non-standard", "https://custom.example.com", false, "https://custom.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAPIURL(tt.url, logger())
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- Explicit mapping validation ----------

func TestValidateExplicitMappings(t *testing.T) {
	valid := []ExplicitMapping{
		{CostCenter: "CC1", PropertyName: "team", PropertyValues: []string{"a"}},
	}
	if err := validateExplicitMappings(valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	missing := []ExplicitMapping{
		{CostCenter: "", PropertyName: "team", PropertyValues: []string{"a"}},
	}
	if err := validateExplicitMappings(missing); err == nil {
		t.Fatal("expected error for missing cost_center")
	}

	noValues := []ExplicitMapping{
		{CostCenter: "CC1", PropertyName: "team", PropertyValues: []string{}},
	}
	if err := validateExplicitMappings(noValues); err == nil {
		t.Fatal("expected error for empty property_values")
	}

	noProp := []ExplicitMapping{
		{CostCenter: "CC1", PropertyName: "", PropertyValues: []string{"a"}},
	}
	if err := validateExplicitMappings(noProp); err == nil {
		t.Fatal("expected error for empty property_name")
	}
}

// ---------- Custom-prop cost center validation ----------

func TestValidateCustomPropCostCenters_Valid(t *testing.T) {
	entries := []CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []CustomPropertyFilter{
				{Property: "team", Value: "backend"},
				{Property: "env", Value: "prod"},
			},
		},
	}
	if err := validateCustomPropCostCenters(entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCustomPropCostCenters_MissingFilterProperty(t *testing.T) {
	entries := []CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []CustomPropertyFilter{
				{Property: "", Value: "backend"},
			},
		},
	}
	if err := validateCustomPropCostCenters(entries); err == nil {
		t.Fatal("expected error for missing filter property")
	}
}

func TestValidateCustomPropCostCenters_MissingFilterValue(t *testing.T) {
	entries := []CustomPropCostCenter{
		{
			Name: "Backend",
			Filters: []CustomPropertyFilter{
				{Property: "team", Value: ""},
			},
		},
	}
	if err := validateCustomPropCostCenters(entries); err == nil {
		t.Fatal("expected error for missing filter value")
	}
}

// ---------- Timestamp save/load round trip ----------

func TestTimestamp_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	yaml := `
github:
  enterprise: "ent"
export_dir: "` + dir + `"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := m.SaveLastRunTimestamp(&ts); err != nil {
		t.Fatalf("SaveLastRunTimestamp: %v", err)
	}
	got, err := m.LoadLastRunTimestamp()
	if err != nil {
		t.Fatalf("LoadLastRunTimestamp: %v", err)
	}
	if got == nil {
		t.Fatal("got nil timestamp")
		return
	}
	if !got.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", got, ts)
	}
}

func TestTimestamp_NoFile(t *testing.T) {
	dir := t.TempDir()
	yaml := `
github:
  enterprise: "ent"
export_dir: "` + dir + `"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := m.LoadLastRunTimestamp()
	if err != nil {
		t.Fatalf("LoadLastRunTimestamp: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---------- Placeholder warnings ----------

func TestCheckConfigWarnings_NoAutoCreate(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "users"
  users:
    no_prus_cost_center_id: "REPLACE_WITH_NO_PRUS_COST_CENTER_ID"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should not panic; warnings go to log.
	m.CheckConfigWarnings()
}

func TestCheckConfigWarnings_AutoCreateSkips(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "users"
  users:
    auto_create: true
    no_prus_cost_center_id: "REPLACE_WITH_NO_PRUS_COST_CENTER_ID"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m.CheckConfigWarnings()
}

// ---------- Summary ----------

func TestSummary_ContainsExpectedKeys(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := m.Summary()
	// Mode-independent keys
	for _, k := range []string{
		"enterprise",
		"api_base_url",
		"cost_center_mode",
		"budgets_enabled",
		"log_level",
		"export_dir",
	} {
		if _, ok := s[k]; !ok {
			t.Errorf("Summary missing key %q", k)
		}
	}
	// Users-mode-specific keys (default mode)
	for _, k := range []string{
		"no_prus_cost_center_id",
		"prus_allowed_cost_center_id",
		"prus_exception_users_count",
		"auto_create",
	} {
		if _, ok := s[k]; !ok {
			t.Errorf("Summary missing users-mode key %q", k)
		}
	}
}

func TestSummary_TeamsModeKeys(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
cost_center:
  mode: "teams"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := m.Summary()
	for _, k := range []string{
		"teams_scope",
		"teams_strategy",
		"teams_auto_create",
		"teams_remove_unmatched_users",
	} {
		if _, ok := s[k]; !ok {
			t.Errorf("Summary missing teams-mode key %q", k)
		}
	}
}

// ---------- Config file not found defaults ----------

func TestLoad_FileNotFound(t *testing.T) {
	t.Setenv("GITHUB_ENTERPRISE", "env-ent")
	m, err := Load("/nonexistent/config.yaml", logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Enterprise != "env-ent" {
		t.Errorf("enterprise = %q", m.Enterprise)
	}
}

// ---------- EnableAutoCreation ----------

func TestEnableAutoCreation(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.AutoCreate {
		t.Fatal("expected auto_create=false initially")
	}
	m.EnableAutoCreation()
	if !m.AutoCreate {
		t.Fatal("expected auto_create=true after EnableAutoCreation()")
	}
}

// ---------- Budgets defaults ----------

func TestLoad_BudgetDefaults(t *testing.T) {
	yaml := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.BudgetProducts) != 2 {
		t.Errorf("expected 2 default budget products, got %d", len(m.BudgetProducts))
	}
	if m.BudgetProducts["copilot"].Amount != 100 {
		t.Errorf("copilot amount = %d", m.BudgetProducts["copilot"].Amount)
	}
}

// ---------- Timestamp file JSON structure ----------

func TestTimestamp_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	yaml := `
github:
  enterprise: "ent"
export_dir: "` + dir + `"
`
	m, err := Load(writeConfig(t, yaml), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := m.SaveLastRunTimestamp(&ts); err != nil {
		t.Fatalf("SaveLastRunTimestamp: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, timestampFileName))
	if err != nil {
		t.Fatalf("reading timestamp file: %v", err)
	}
	var td timestampData
	if err := json.Unmarshal(data, &td); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if td.LastRun != "2025-01-01T00:00:00Z" {
		t.Errorf("last_run = %q", td.LastRun)
	}
	if td.SavedAt == "" {
		t.Error("saved_at is empty")
	}
}

// ---------- Helper tests ----------

func TestDefaultString(t *testing.T) {
	if got := defaultString("a", "b"); got != "a" {
		t.Errorf("got %q, want %q", got, "a")
	}
	if got := defaultString("", "b"); got != "b" {
		t.Errorf("got %q, want %q", got, "b")
	}
	if got := defaultString("", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestEnvOrFallback(t *testing.T) {
	t.Setenv("TEST_ENV_OR_FALLBACK_KEY", "env-val")
	if got := envOrFallback("TEST_ENV_OR_FALLBACK_KEY", "yaml-val"); got != "env-val" {
		t.Errorf("got %q, want env-val", got)
	}
	if got := envOrFallback("UNSET_KEY_12345", "yaml-val"); got != "yaml-val" {
		t.Errorf("got %q, want yaml-val", got)
	}
}

// ---------- Repo custom property definitions ----------

func TestLoad_RepoCustomProperties_Valid(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
repo_custom_properties:
  - name: "team"
    value_type: "single_select"
    required: true
    description: "Owning team"
    allowed_values:
      - "backend"
      - "frontend"
  - name: "environment"
    value_type: "multi_select"
    allowed_values:
      - "production"
      - "staging"
  - name: "cost-center-id"
    value_type: "string"
    required: false
    description: "Direct cost center ID"
  - name: "archived"
    value_type: "true_false"
`
	m, err := Load(writeConfig(t, cfg), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.RepoCustomProperties) != 4 {
		t.Fatalf("expected 4 custom property defs, got %d", len(m.RepoCustomProperties))
	}

	team := m.RepoCustomProperties[0]
	if team.Name != "team" {
		t.Errorf("name = %q, want %q", team.Name, "team")
	}
	if team.ValueType != "single_select" {
		t.Errorf("value_type = %q, want single_select", team.ValueType)
	}
	if !team.Required {
		t.Error("expected required = true")
	}
	if team.Description != "Owning team" {
		t.Errorf("description = %q", team.Description)
	}
	if len(team.AllowedValues) != 2 {
		t.Errorf("expected 2 allowed_values, got %d", len(team.AllowedValues))
	}

	env := m.RepoCustomProperties[1]
	if env.ValueType != "multi_select" {
		t.Errorf("value_type = %q, want multi_select", env.ValueType)
	}

	costCenter := m.RepoCustomProperties[2]
	if costCenter.ValueType != "string" {
		t.Errorf("value_type = %q, want string", costCenter.ValueType)
	}

	archived := m.RepoCustomProperties[3]
	if archived.ValueType != "true_false" {
		t.Errorf("value_type = %q, want true_false", archived.ValueType)
	}
}

func TestLoad_RepoCustomProperties_Absent(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, cfg), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.RepoCustomProperties) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(m.RepoCustomProperties))
	}
}

func TestLoad_RepoCustomProperties_DefaultValue(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
repo_custom_properties:
  - name: "env"
    value_type: "single_select"
    default_value: "development"
    allowed_values:
      - "development"
      - "production"
`
	m, err := Load(writeConfig(t, cfg), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.RepoCustomProperties[0].DefaultValue != "development" {
		t.Errorf("default_value = %q, want %q", m.RepoCustomProperties[0].DefaultValue, "development")
	}
}

func TestValidateRepoCustomProperties_MissingName(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "", ValueType: "string"},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidateRepoCustomProperties_MissingValueType(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "team", ValueType: ""},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for missing value_type")
	}
}

func TestValidateRepoCustomProperties_InvalidValueType(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "team", ValueType: "dropdown"},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for invalid value_type")
	}
}

func TestValidateRepoCustomProperties_DuplicateName(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "team", ValueType: "string"},
		{Name: "team", ValueType: "string"},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestValidateRepoCustomProperties_AllowedValuesOnNonSelectType(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "flag", ValueType: "true_false", AllowedValues: []string{"yes", "no"}},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for allowed_values on true_false type")
	}
}

func TestValidateRepoCustomProperties_AllowedValuesOnStringType(t *testing.T) {
	defs := []RepoCustomPropertyDef{
		{Name: "note", ValueType: "string", AllowedValues: []string{"a"}},
	}
	if err := validateRepoCustomProperties(defs); err == nil {
		t.Fatal("expected error for allowed_values on string type")
	}
}

func TestValidateRepoCustomProperties_SelectTypeMissingAllowedValues(t *testing.T) {
	tests := []struct {
		name      string
		valueType string
	}{
		{"single_select without allowed_values", "single_select"},
		{"multi_select without allowed_values", "multi_select"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := []RepoCustomPropertyDef{
				{Name: "prop", ValueType: tt.valueType},
			}
			if err := validateRepoCustomProperties(defs); err == nil {
				t.Fatalf("expected error for %s without allowed_values", tt.valueType)
			}
		})
	}
}

func TestValidateRepoCustomProperties_AllValidTypes(t *testing.T) {
	tests := []struct {
		name      string
		valueType string
		allowed   []string
	}{
		{"string", "string", nil},
		{"true_false", "true_false", nil},
		{"single_select", "single_select", []string{"a", "b"}},
		{"multi_select", "multi_select", []string{"x", "y"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := []RepoCustomPropertyDef{
				{Name: "prop", ValueType: tt.valueType, AllowedValues: tt.allowed},
			}
			if err := validateRepoCustomProperties(defs); err != nil {
				t.Fatalf("unexpected error for valid type %q: %v", tt.valueType, err)
			}
		})
	}
}

func TestLoad_RepoCustomProperties_InvalidYAML(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
repo_custom_properties:
  - name: "flag"
    value_type: "true_false"
    allowed_values:
      - "yes"
`
	_, err := Load(writeConfig(t, cfg), logger())
	if err == nil {
		t.Fatal("expected error for allowed_values on true_false type")
	}
}

func TestSummary_IncludesRepoCustomPropertiesCount(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
repo_custom_properties:
  - name: "team"
    value_type: "string"
  - name: "env"
    value_type: "single_select"
    allowed_values:
      - "prod"
      - "staging"
`
	m, err := Load(writeConfig(t, cfg), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := m.Summary()
	count, ok := s["repo_custom_properties_count"]
	if !ok {
		t.Fatal("Summary missing key repo_custom_properties_count")
	}
	if count != 2 {
		t.Errorf("repo_custom_properties_count = %v, want 2", count)
	}
}

func TestSummary_NoRepoCustomPropertiesKey_WhenEmpty(t *testing.T) {
	cfg := `
github:
  enterprise: "ent"
`
	m, err := Load(writeConfig(t, cfg), logger())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := m.Summary()
	if _, ok := s["repo_custom_properties_count"]; ok {
		t.Error("Summary should not include repo_custom_properties_count when empty")
	}
}
