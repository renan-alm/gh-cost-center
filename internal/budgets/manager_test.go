package budgets

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/renan-alm/gh-cost-center/internal/config"
	"github.com/renan-alm/gh-cost-center/internal/github"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestClient(t *testing.T, url string) *github.Client {
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

func TestNewManager(t *testing.T) {
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
		"copilot": {Amount: 200, Enabled: false},
	}

	mgr := NewManager(nil, testLogger(), products)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
		return
	}
	if len(mgr.products) != 2 {
		t.Errorf("expected 2 products, got %d", len(mgr.products))
	}
}

func TestIsAvailable_Initially(t *testing.T) {
	mgr := NewManager(nil, testLogger(), nil)

	if !mgr.IsAvailable() {
		t.Error("expected IsAvailable() == true initially")
	}
}

func TestIsAvailable_AfterUnavailable(t *testing.T) {
	mgr := NewManager(nil, testLogger(), nil)
	mgr.unavailable = true

	if mgr.IsAvailable() {
		t.Error("expected IsAvailable() == false after marking unavailable")
	}
}

func TestEnsureBudgets_SkipsWhenUnavailable(t *testing.T) {
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := NewManager(nil, testLogger(), products)
	mgr.unavailable = true

	// Should return immediately without panic (no client set).
	if err := mgr.EnsureBudgetsForCostCenter("cc-id-1", "Test CC"); err != nil {
		t.Errorf("expected nil error when unavailable, got %v", err)
	}
}

func TestEnsureBudgets_AllSucceed(t *testing.T) {
	// Server that returns 200 for budget list (empty) and 201 for creates.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"budgets": []any{}})
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
		"copilot": {Amount: 200, Enabled: true},
	}
	mgr := NewManager(client, testLogger(), products)

	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !mgr.IsAvailable() {
		t.Error("manager should remain available after success")
	}
}

func TestEnsureBudgets_SkipsDisabledProducts(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"budgets": []any{}})
			return
		}
		callCount++
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
		"copilot": {Amount: 200, Enabled: false}, // disabled
	}
	mgr := NewManager(client, testLogger(), products)

	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	// Only "actions" POST should have been made (GET for list check + POST for create = per product).
	if callCount != 1 {
		t.Errorf("expected 1 budget create call (only enabled product), got %d", callCount)
	}
}

func TestEnsureBudgets_PartialFailureAccumulates(t *testing.T) {
	// Server returns 500 for every POST (simulating product creation failure).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"budgets": []any{}})
			return
		}
		// Return 500 (server error) — triggers retry exhaustion.
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
		"copilot": {Amount: 200, Enabled: true},
	}
	mgr := NewManager(client, testLogger(), products)

	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Test CC")
	if err == nil {
		t.Fatal("expected error for partial failures")
	}
	if !strings.Contains(err.Error(), "budget creation failed for cost center Test CC") {
		t.Errorf("unexpected error message: %v", err)
	}
	// Should still be available — only API unavailable (404) disables.
	if !mgr.IsAvailable() {
		t.Error("manager should remain available after non-404 failures")
	}
}

func TestEnsureBudgets_APIUnavailable_GracefulDegradation(t *testing.T) {
	// Server returns 404 to simulate budgets API not enabled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := NewManager(client, testLogger(), products)

	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error for API unavailable (graceful degradation), got %v", err)
	}
	if mgr.IsAvailable() {
		t.Error("manager should be marked unavailable after 404")
	}
}

func TestEnsureBudgets_MixedSuccessAndFailure(t *testing.T) {
	// Use a deterministic single product to avoid map ordering issues.
	// First GET returns empty budgets; POST returns 400.
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

	client := newTestClient(t, srv.URL)
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: true},
	}
	mgr := NewManager(client, testLogger(), products)

	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Fail CC")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "actions") {
		t.Errorf("error should mention failed product 'actions': %v", err)
	}
}

func TestEnsureBudgets_NoEnabledProducts(t *testing.T) {
	products := map[string]config.ProductBudget{
		"actions": {Amount: 100, Enabled: false},
		"copilot": {Amount: 200, Enabled: false},
	}
	mgr := NewManager(nil, testLogger(), products)

	// No client needed since nothing should be called.
	err := mgr.EnsureBudgetsForCostCenter("cc-1", "Test CC")
	if err != nil {
		t.Errorf("expected nil error when all products disabled, got %v", err)
	}
}

// Ensure the test client builder uses a short timeout so tests don't hang.
var _ = time.Second
