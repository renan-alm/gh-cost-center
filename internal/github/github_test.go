package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renan-alm/gh-cost-center/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&discardW{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type discardW struct{}

func (discardW) Write(p []byte) (int, error) { return len(p), nil }

func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	return &Client{
		http:       &http.Client{Timeout: 5 * time.Second},
		baseURL:    url,
		enterprise: "test-ent",
		log:        testLogger(),
	}
}

func TestNewClient(t *testing.T) {
	logger := testLogger()
	t.Run("success", func(t *testing.T) {
		cfg := &config.Manager{Enterprise: "my-ent", APIBaseURL: "https://api.github.com"}
		c, err := NewClient(cfg, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.enterprise != "my-ent" {
			t.Errorf("enterprise = %q, want %q", c.enterprise, "my-ent")
		}
		if c.baseURL != "https://api.github.com" {
			t.Errorf("baseURL = %q", c.baseURL)
		}
	})
	t.Run("trailing slash stripped", func(t *testing.T) {
		cfg := &config.Manager{Enterprise: "ent", APIBaseURL: "https://api.github.com/"}
		c, err := NewClient(cfg, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.baseURL != "https://api.github.com" {
			t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
		}
	})
	t.Run("empty enterprise", func(t *testing.T) {
		cfg := &config.Manager{Enterprise: "", APIBaseURL: "https://api.github.com"}
		_, err := NewClient(cfg, logger)
		if err == nil {
			t.Fatal("expected error for empty enterprise")
		}
	})
}

func TestEnterpriseURL(t *testing.T) {
	c := &Client{baseURL: "https://api.github.com", enterprise: "my-ent"}
	tests := []struct {
		path, want string
	}{
		{"/copilot/billing/seats", "https://api.github.com/enterprises/my-ent/copilot/billing/seats"},
		{"/settings/billing/cost-centers", "https://api.github.com/enterprises/my-ent/settings/billing/cost-centers"},
		{"/teams", "https://api.github.com/enterprises/my-ent/teams"},
	}
	for _, tt := range tests {
		if got := c.enterpriseURL(tt.path); got != tt.want {
			t.Errorf("enterpriseURL(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestAPIError(t *testing.T) {
	e := &APIError{StatusCode: 404, Body: "not found"}
	if !strings.Contains(e.Error(), "404") {
		t.Errorf("Error() missing status code: %s", e.Error())
	}
	if !strings.Contains(e.Error(), "not found") {
		t.Errorf("Error() missing body: %s", e.Error())
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("connection reset by peer"), true},
		{fmt.Errorf("i/o timeout"), true},
		{fmt.Errorf("TLS handshake timeout"), true},
		{fmt.Errorf("unexpected EOF"), true},
		{fmt.Errorf("404: not found"), false},
		{fmt.Errorf("permission denied"), false},
	}
	for _, tt := range tests {
		label := "<nil>"
		if tt.err != nil {
			label = tt.err.Error()
		}
		if got := isTransient(tt.err); got != tt.want {
			t.Errorf("isTransient(%q) = %v, want %v", label, got, tt.want)
		}
	}
}

func TestBackoff(t *testing.T) {
	c := &Client{log: testLogger()}
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
	}
	for _, tt := range tests {
		if got := c.backoff(tt.attempt, nil); got != tt.want {
			t.Errorf("backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestRateLimitWait(t *testing.T) {
	c := &Client{log: testLogger()}
	t.Run("with valid header", func(t *testing.T) {
		resetTime := time.Now().Add(30 * time.Second)
		resp := &http.Response{Header: http.Header{"X-Ratelimit-Reset": []string{strconv.FormatInt(resetTime.Unix(), 10)}}}
		wait := c.rateLimitWait(resp)
		if wait < 29*time.Second || wait > 33*time.Second {
			t.Errorf("rateLimitWait = %v, expected ~31s", wait)
		}
	})
	t.Run("missing header", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		if wait := c.rateLimitWait(resp); wait != rateLimitFallback {
			t.Errorf("rateLimitWait = %v, want %v", wait, rateLimitFallback)
		}
	})
	t.Run("invalid header", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{"X-Ratelimit-Reset": []string{"bad"}}}
		if wait := c.rateLimitWait(resp); wait != rateLimitFallback {
			t.Errorf("rateLimitWait = %v, want %v", wait, rateLimitFallback)
		}
	})
	t.Run("past reset time", func(t *testing.T) {
		resetTime := time.Now().Add(-10 * time.Second)
		resp := &http.Response{Header: http.Header{"X-Ratelimit-Reset": []string{strconv.FormatInt(resetTime.Unix(), 10)}}}
		if wait := c.rateLimitWait(resp); wait != time.Second {
			t.Errorf("rateLimitWait = %v, want 1s", wait)
		}
	})
}

func TestDoJSON_Success(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != acceptHeader {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("X-GitHub-Api-Version") != apiVersion {
			t.Errorf("X-GitHub-Api-Version = %q", r.Header.Get("X-GitHub-Api-Version"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload{Name: "Alice", Age: 30})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	var got payload
	if _, err := c.doJSON(http.MethodGet, srv.URL+"/test", nil, &got); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if got.Name != "Alice" || got.Age != 30 {
		t.Errorf("got %+v, want {Alice 30}", got)
	}
}

func TestDoJSON_NoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	if _, err := c.doJSON(http.MethodPost, srv.URL+"/test", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
}

func TestDoJSON_PostWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-cc" {
			t.Errorf("name = %q", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "abc-123"})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	var resp map[string]string
	if _, err := c.doJSON(http.MethodPost, srv.URL+"/test", map[string]string{"name": "test-cc"}, &resp); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if resp["id"] != "abc-123" {
		t.Errorf("id = %q", resp["id"])
	}
}

func TestDoJSON_NonRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.doJSON(http.MethodGet, srv.URL+"/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "forbidden") {
		t.Errorf("Body = %q", apiErr.Body)
	}
}

func TestDoJSON_RetryOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	var resp map[string]string
	if _, err := c.doJSON(http.MethodGet, srv.URL+"/test", nil, &resp); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q", resp["status"])
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestDoJSON_ExhaustedRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.doJSON(http.MethodGet, srv.URL+"/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if got := calls.Load(); got != int32(maxRetries) {
		t.Errorf("calls = %d, want %d", got, maxRetries)
	}
}

func TestReadBody(t *testing.T) {
	t.Run("nil body", func(t *testing.T) {
		if got := readBody(&http.Response{Body: nil}); got != "" {
			t.Errorf("readBody(nil) = %q", got)
		}
	})
	t.Run("NoBody", func(t *testing.T) {
		if got := readBody(&http.Response{Body: http.NoBody}); got != "" {
			t.Errorf("readBody(NoBody) = %q", got)
		}
	})
}

func TestDeduplicateUsers(t *testing.T) {
	users := []CopilotUser{
		{Login: "alice"}, {Login: "bob"}, {Login: "alice"},
		{Login: ""}, {Login: "charlie"}, {Login: "bob"},
	}
	got := deduplicateUsers(users, testLogger())
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	logins := make(map[string]bool)
	for _, u := range got {
		logins[u.Login] = true
	}
	for _, want := range []string{"alice", "bob", "charlie"} {
		if !logins[want] {
			t.Errorf("missing %q", want)
		}
	}
}

func TestDeduplicateUsers_Empty(t *testing.T) {
	if got := deduplicateUsers(nil, testLogger()); len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilterUsersByTimestamp(t *testing.T) {
	threshold := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	users := []CopilotUser{
		{Login: "old", CreatedAt: "2024-05-01T00:00:00Z"},
		{Login: "new1", CreatedAt: "2024-07-01T12:00:00Z"},
		{Login: "new2", CreatedAt: "2024-06-02T00:00:00Z"},
		{Login: "exact", CreatedAt: "2024-06-01T00:00:00Z"},
		{Login: "empty", CreatedAt: ""},
		{Login: "bad", CreatedAt: "not-a-date"},
	}
	got := FilterUsersByTimestamp(users, threshold)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	logins := make(map[string]bool)
	for _, u := range got {
		logins[u.Login] = true
	}
	if !logins["new1"] || !logins["new2"] {
		t.Errorf("logins = %v", logins)
	}
}

func TestToSet(t *testing.T) {
	s := toSet([]string{"a", "b", "c", "b"})
	if len(s) != 3 {
		t.Errorf("len = %d, want 3", len(s))
	}
	for _, k := range []string{"a", "b", "c"} {
		if !s[k] {
			t.Errorf("missing %q", k)
		}
	}
}

func TestToSet_Empty(t *testing.T) {
	if s := toSet(nil); len(s) != 0 {
		t.Errorf("len = %d, want 0", len(s))
	}
}

func TestUUIDFromConflictRe(t *testing.T) {
	tests := []struct {
		name, input, wantID string
		wantLen             int
	}{
		{"standard", "Existing cost center UUID: d1e2f3a4-b5c6-7890-abcd-ef1234567890", "d1e2f3a4-b5c6-7890-abcd-ef1234567890", 2},
		{"case insensitive", "Existing Cost Center UUID: A1B2C3D4-E5F6-7890-ABCD-EF1234567890", "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", 2},
		{"no match", "Some unrelated error message", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := uuidFromConflictRe.FindStringSubmatch(tt.input)
			if len(m) != tt.wantLen {
				t.Fatalf("match len = %d, want %d", len(m), tt.wantLen)
			}
			if tt.wantLen == 2 && !strings.EqualFold(m[1], tt.wantID) {
				t.Errorf("UUID = %q, want %q", m[1], tt.wantID)
			}
		})
	}
}

func TestGetBudgetTypeAndSKU(t *testing.T) {
	tests := []struct {
		product, wantType, wantSKU string
	}{
		{"actions", "ProductPricing", "actions"},
		{"COPILOT", "ProductPricing", "copilot"},
		{"Packages", "ProductPricing", "packages"},
		{"codespaces", "ProductPricing", "codespaces"},
		{"ghas", "ProductPricing", "ghas"},
		{"ghec", "ProductPricing", "ghec"},
		{"copilot_premium_request", "SkuPricing", "copilot_premium_request"},
		{"copilot_agent_premium_request", "SkuPricing", "copilot_agent_premium_request"},
		{"copilot_enterprise", "SkuPricing", "copilot_enterprise"},
		{"actions_linux", "SkuPricing", "actions_linux"},
		{"ghas_licenses", "SkuPricing", "ghas_licenses"},
		{"ghec_licenses", "SkuPricing", "ghec_licenses"},
		{"git_lfs_storage", "SkuPricing", "git_lfs_storage"},
		{"models_inference", "SkuPricing", "models_inference"},
		{"unknown_product", "SkuPricing", "unknown_product"},
	}
	for _, tt := range tests {
		t.Run(tt.product, func(t *testing.T) {
			gotType, gotSKU := GetBudgetTypeAndSKU(tt.product)
			if gotType != tt.wantType {
				t.Errorf("type = %q, want %q", gotType, tt.wantType)
			}
			if gotSKU != tt.wantSKU {
				t.Errorf("sku = %q, want %q", gotSKU, tt.wantSKU)
			}
		})
	}
}

func TestGetCopilotUsers_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pg := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch pg {
		case "1", "":
			// Return exactly per_page items so the client fetches page 2.
			seats := make([]seatEntry, 100)
			for i := range seats {
				seats[i] = seatEntry{Assignee: assignee{Login: fmt.Sprintf("user-%d", i), ID: int64(i)}}
			}
			_ = json.NewEncoder(w).Encode(seatsResponse{TotalSeats: 103, Seats: seats})
		case "2":
			_ = json.NewEncoder(w).Encode(seatsResponse{TotalSeats: 103, Seats: []seatEntry{
				{Assignee: assignee{Login: "alice", ID: 100}},
				{Assignee: assignee{Login: "bob", ID: 101}},
				{Assignee: assignee{Login: "charlie", ID: 102}},
			}})
		default:
			t.Errorf("unexpected page %q", pg)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	users, err := c.GetCopilotUsers()
	if err != nil {
		t.Fatalf("GetCopilotUsers: %v", err)
	}
	if len(users) != 103 {
		t.Fatalf("got %d users, want 103", len(users))
	}
}

func TestGetAllActiveCostCenters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(costCentersListResponse{CostCenters: []CostCenter{
			{ID: "cc-1", Name: "No PRU", State: "active"},
			{ID: "cc-2", Name: "PRU Allowed", State: "active"},
			{ID: "cc-3", Name: "Deleted", State: "deleted"},
			{ID: "", Name: "Bad", State: "active"},
		}})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	active, err := c.GetAllActiveCostCenters()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("got %d, want 2", len(active))
	}
	if active["No PRU"] != "cc-1" {
		t.Errorf("No PRU = %q", active["No PRU"])
	}
}

func TestCreateCostCenter_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(costCenterCreateResponse{ID: "new-id", Name: "CC"})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	id, err := c.CreateCostCenter("CC")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "new-id" {
		t.Errorf("id = %q", id)
	}
}

func TestCreateCostCenter_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("Existing cost center UUID: d1e2f3a4-b5c6-7890-abcd-ef1234567890"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	id, err := c.CreateCostCenter("Existing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "d1e2f3a4-b5c6-7890-abcd-ef1234567890" {
		t.Errorf("id = %q", id)
	}
}

func TestListBudgets_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.ListBudgets()
	if err == nil {
		t.Fatal("expected error")
	}
	var unavail *BudgetsAPIUnavailableError
	if !errors.As(err, &unavail) {
		t.Fatalf("expected BudgetsAPIUnavailableError, got %T", err)
	}
}

func TestListBudgets_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(budgetsListResponse{Budgets: []Budget{
			{BudgetType: "SkuPricing", BudgetProductSKU: "copilot_premium_request", BudgetScope: "cost_center", BudgetAmount: 100, BudgetEntityName: "cc-1"},
		}})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	budgets, err := c.ListBudgets()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(budgets) != 1 {
		t.Fatalf("got %d, want 1", len(budgets))
	}
	if budgets[0].BudgetAmount != 100 {
		t.Errorf("amount = %d", budgets[0].BudgetAmount)
	}
}

func TestGetOrgTeams_Pagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			_ = json.NewEncoder(w).Encode([]Team{{ID: 1, Name: "A", Slug: "a"}, {ID: 2, Name: "B", Slug: "b"}})
		case 2:
			_ = json.NewEncoder(w).Encode([]Team{})
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	teams, err := c.GetOrgTeams("my-org")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("got %d, want 2", len(teams))
	}
}

func TestGetOrgPropertySchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]PropertyDefinition{
			{PropertyName: "cost-center", ValueType: "single_select"},
			{PropertyName: "team", ValueType: "string"},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	defs, err := c.GetOrgPropertySchema("my-org")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("got %d, want 2", len(defs))
	}
	if defs[0].PropertyName != "cost-center" {
		t.Errorf("first = %q", defs[0].PropertyName)
	}
}
