package pru

import (
	"log/slog"
	"os"
	"testing"

	"github.com/renan-alm/gh-cost-center/internal/config"
	"github.com/renan-alm/gh-cost-center/internal/github"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testConfig(noPRUID, pruAllowedID string, exceptions []string) *config.Manager {
	return &config.Manager{
		Enterprise:                "test-enterprise",
		APIBaseURL:                "https://api.github.com",
		NoPRUsCostCenterID:        noPRUID,
		PRUsAllowedCostCenterID:   pruAllowedID,
		PRUsExceptionUsers:        exceptions,
		NoPRUsCostCenterName:      "No PRU",
		PRUsAllowedCostCenterName: "PRU Allowed",
	}
}

func TestAssignCostCenter_ExceptionUser(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"alice", "bob"})
	mgr := NewManager(cfg, testLogger())

	user := github.CopilotUser{Login: "alice"}
	got := mgr.AssignCostCenter(user)

	if got != "cc-pru-allowed" {
		t.Errorf("AssignCostCenter(alice) = %q; want %q", got, "cc-pru-allowed")
	}
}

func TestAssignCostCenter_RegularUser(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"alice"})
	mgr := NewManager(cfg, testLogger())

	user := github.CopilotUser{Login: "charlie"}
	got := mgr.AssignCostCenter(user)

	if got != "cc-no-pru" {
		t.Errorf("AssignCostCenter(charlie) = %q; want %q", got, "cc-no-pru")
	}
}

func TestAssignCostCenter_CaseInsensitive(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"Alice"})
	mgr := NewManager(cfg, testLogger())

	tests := []struct {
		login string
		want  string
	}{
		{"alice", "cc-pru-allowed"},
		{"ALICE", "cc-pru-allowed"},
		{"Alice", "cc-pru-allowed"},
		{"bob", "cc-no-pru"},
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			got := mgr.AssignCostCenter(github.CopilotUser{Login: tt.login})
			if got != tt.want {
				t.Errorf("AssignCostCenter(%s) = %q; want %q", tt.login, got, tt.want)
			}
		})
	}
}

func TestIsException(t *testing.T) {
	cfg := testConfig("cc1", "cc2", []string{"alice", "Bob"})
	mgr := NewManager(cfg, testLogger())

	if !mgr.IsException("alice") {
		t.Error("IsException(alice) = false; want true")
	}
	if !mgr.IsException("ALICE") {
		t.Error("IsException(ALICE) = false; want true")
	}
	if !mgr.IsException("bob") {
		t.Error("IsException(bob) = false; want true")
	}
	if mgr.IsException("charlie") {
		t.Error("IsException(charlie) = true; want false")
	}
}

func TestAssignmentGroups(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"alice"})
	mgr := NewManager(cfg, testLogger())

	users := []github.CopilotUser{
		{Login: "alice"},
		{Login: "bob"},
		{Login: "charlie"},
	}

	groups := mgr.AssignmentGroups(users)

	if len(groups["cc-pru-allowed"]) != 1 {
		t.Errorf("PRU-allowed group has %d users; want 1", len(groups["cc-pru-allowed"]))
	}
	if groups["cc-pru-allowed"][0] != "alice" {
		t.Errorf("PRU-allowed group[0] = %q; want alice", groups["cc-pru-allowed"][0])
	}
	if len(groups["cc-no-pru"]) != 2 {
		t.Errorf("No-PRU group has %d users; want 2", len(groups["cc-no-pru"]))
	}
}

func TestAssignmentGroups_NoExceptions(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{})
	mgr := NewManager(cfg, testLogger())

	users := []github.CopilotUser{
		{Login: "alice"},
		{Login: "bob"},
	}

	groups := mgr.AssignmentGroups(users)

	if len(groups["cc-pru-allowed"]) != 0 {
		t.Errorf("PRU-allowed group has %d users; want 0", len(groups["cc-pru-allowed"]))
	}
	if len(groups["cc-no-pru"]) != 2 {
		t.Errorf("No-PRU group has %d users; want 2", len(groups["cc-no-pru"]))
	}
}

func TestAssignmentGroups_AllExceptions(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"alice", "bob"})
	mgr := NewManager(cfg, testLogger())

	users := []github.CopilotUser{
		{Login: "alice"},
		{Login: "bob"},
	}

	groups := mgr.AssignmentGroups(users)

	if len(groups["cc-pru-allowed"]) != 2 {
		t.Errorf("PRU-allowed group has %d users; want 2", len(groups["cc-pru-allowed"]))
	}
	if len(groups["cc-no-pru"]) != 0 {
		t.Errorf("No-PRU group has %d users; want 0", len(groups["cc-no-pru"]))
	}
}

func TestGenerateSummary(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{"alice"})
	mgr := NewManager(cfg, testLogger())

	users := []github.CopilotUser{
		{Login: "alice"},
		{Login: "bob"},
		{Login: "charlie"},
		{Login: "dave"},
	}

	summary := mgr.GenerateSummary(users)

	if summary["cc-pru-allowed"] != 1 {
		t.Errorf("summary[cc-pru-allowed] = %d; want 1", summary["cc-pru-allowed"])
	}
	if summary["cc-no-pru"] != 3 {
		t.Errorf("summary[cc-no-pru] = %d; want 3", summary["cc-no-pru"])
	}
}

func TestGenerateSummary_Empty(t *testing.T) {
	cfg := testConfig("cc-no-pru", "cc-pru-allowed", []string{})
	mgr := NewManager(cfg, testLogger())

	summary := mgr.GenerateSummary(nil)

	if len(summary) != 0 {
		t.Errorf("summary has %d entries; want 0", len(summary))
	}
}

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		noPRU      string
		pruAllowed string
		wantIssues int
	}{
		{"valid", "cc1", "cc2", 0},
		{"missing_no_pru", "", "cc2", 1},
		{"missing_pru_allowed", "cc1", "", 1},
		{"both_missing", "", "", 2},
		{"same_ids", "cc1", "cc1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(tt.noPRU, tt.pruAllowed, nil)
			mgr := NewManager(cfg, testLogger())
			issues := mgr.ValidateConfiguration()
			if len(issues) != tt.wantIssues {
				t.Errorf("ValidateConfiguration() returned %d issues; want %d: %v",
					len(issues), tt.wantIssues, issues)
			}
		})
	}
}

func TestSetCostCenterIDs(t *testing.T) {
	cfg := testConfig("old-no-pru", "old-pru-allowed", []string{})
	mgr := NewManager(cfg, testLogger())

	mgr.SetCostCenterIDs("new-no-pru", "new-pru-allowed")

	user := github.CopilotUser{Login: "alice"}
	got := mgr.AssignCostCenter(user)
	if got != "new-no-pru" {
		t.Errorf("after SetCostCenterIDs, AssignCostCenter = %q; want new-no-pru", got)
	}

	if mgr.NoPRUCCID() != "new-no-pru" {
		t.Errorf("NoPRUCCID() = %q; want new-no-pru", mgr.NoPRUCCID())
	}
	if mgr.PRUAllowedCCID() != "new-pru-allowed" {
		t.Errorf("PRUAllowedCCID() = %q; want new-pru-allowed", mgr.PRUAllowedCCID())
	}
}

func TestNewManager_NilExceptions(t *testing.T) {
	cfg := testConfig("cc1", "cc2", nil)
	mgr := NewManager(cfg, testLogger())

	if mgr.IsException("anyone") {
		t.Error("IsException should return false when exception list is nil")
	}
}
