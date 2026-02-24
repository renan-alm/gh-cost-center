package github

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// BudgetsAPIUnavailableError indicates the GitHub Budgets API is not enabled
// for this enterprise.
type BudgetsAPIUnavailableError struct {
	Enterprise string
}

func (e *BudgetsAPIUnavailableError) Error() string {
	return fmt.Sprintf("Budgets API is not available for enterprise %q; this feature may not be enabled", e.Enterprise)
}

// Budget represents a single budget entry from the API.
type Budget struct {
	BudgetType       string `json:"budget_type"`
	BudgetProductSKU string `json:"budget_product_sku"`
	BudgetScope      string `json:"budget_scope"`
	BudgetAmount     int    `json:"budget_amount"`
	BudgetEntityName string `json:"budget_entity_name"`
}

// budgetsListResponse is the JSON envelope for the budgets list endpoint.
type budgetsListResponse struct {
	Budgets []Budget `json:"budgets"`
}

// ListBudgets returns all budgets for the enterprise.
func (c *Client) ListBudgets() ([]Budget, error) {
	url := c.enterpriseURL("/settings/billing/budgets")
	var resp budgetsListResponse
	_, err := c.doJSON(http.MethodGet, url, nil, &resp)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, &BudgetsAPIUnavailableError{Enterprise: c.enterprise}
		}
		return nil, fmt.Errorf("listing budgets: %w", err)
	}
	return resp.Budgets, nil
}

// CheckCostCenterHasBudget returns true if any budget targets the given cost
// center name.  Due to a known API bug, the entity name may store the CC name
// rather than the UUID, so we compare against both.
func (c *Client) CheckCostCenterHasBudget(costCenterID, costCenterName string) (bool, error) {
	budgets, err := c.ListBudgets()
	if err != nil {
		return false, err
	}
	for _, b := range budgets {
		if b.BudgetScope == "cost_center" &&
			(b.BudgetEntityName == costCenterName || b.BudgetEntityName == costCenterID) {
			c.log.Debug("Budget already exists for cost center",
				"cost_center_name", costCenterName, "cost_center_id", costCenterID)
			return true, nil
		}
	}
	return false, nil
}

// CheckCostCenterHasProductBudget returns true if a budget exists for the
// given cost center and product combination.
func (c *Client) CheckCostCenterHasProductBudget(costCenterID, costCenterName, product string) (bool, error) {
	budgets, err := c.ListBudgets()
	if err != nil {
		return false, err
	}
	_, sku := GetBudgetTypeAndSKU(product)
	for _, b := range budgets {
		if b.BudgetScope == "cost_center" &&
			(b.BudgetEntityName == costCenterID || b.BudgetEntityName == costCenterName) &&
			b.BudgetProductSKU == sku {
			c.log.Info("Found existing budget", "product", product, "cost_center", costCenterName)
			return true, nil
		}
	}
	return false, nil
}

// CreateBudget creates a default Copilot Premium Request budget for a cost
// center.  If a budget already exists it returns true without error.
func (c *Client) CreateBudget(costCenterID, costCenterName string, amount int) (bool, error) {
	exists, err := c.CheckCostCenterHasBudget(costCenterID, costCenterName)
	if err != nil {
		return false, err
	}
	if exists {
		c.log.Info("Budget already exists", "cost_center", costCenterName, "cost_center_id", costCenterID)
		return true, nil
	}

	return c.createBudgetRequest(costCenterID, costCenterName, "SkuPricing", "copilot_premium_request", amount)
}

// CreateProductBudget creates a product-specific budget for a cost center.
func (c *Client) CreateProductBudget(costCenterID, costCenterName, product string, amount int) (bool, error) {
	exists, err := c.CheckCostCenterHasProductBudget(costCenterID, costCenterName, product)
	if err != nil {
		return false, err
	}
	if exists {
		c.log.Info("Product budget already exists",
			"product", product, "cost_center", costCenterName)
		return true, nil
	}

	budgetType, sku := GetBudgetTypeAndSKU(product)
	return c.createBudgetRequest(costCenterID, costCenterName, budgetType, sku, amount)
}

// createBudgetRequest sends the POST to create a budget.
func (c *Client) createBudgetRequest(costCenterID, costCenterName, budgetType, productSKU string, amount int) (bool, error) {
	url := c.enterpriseURL("/settings/billing/budgets")

	body := map[string]any{
		"budget_type":           budgetType,
		"budget_product_sku":    productSKU,
		"budget_scope":          "cost_center",
		"budget_amount":         amount,
		"prevent_further_usage": true,
		"budget_entity_name":    costCenterID,
		"budget_alerting": map[string]any{
			"will_alert":       false,
			"alert_recipients": []string{},
		},
	}

	_, err := c.doJSON(http.MethodPost, url, body, nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return false, &BudgetsAPIUnavailableError{Enterprise: c.enterprise}
		}
		return false, fmt.Errorf("creating budget for cost center %q: %w", costCenterName, err)
	}

	c.log.Info("Successfully created budget",
		"cost_center", costCenterName, "product_sku", productSKU, "amount", amount)
	return true, nil
}

// GetBudgetTypeAndSKU maps a product name to the appropriate (budgetType,
// productSKU) tuple.  Product-level identifiers use "ProductPricing", while
// SKU-level identifiers use "SkuPricing".
//
// Reference: https://docs.github.com/enterprise-cloud@latest/billing/reference/product-and-sku-names
func GetBudgetTypeAndSKU(product string) (budgetType, productSKU string) {
	p := strings.ToLower(product)

	// Product-level identifiers (ProductPricing).
	productLevel := map[string]string{
		"actions":    "actions",
		"packages":   "packages",
		"codespaces": "codespaces",
		"copilot":    "copilot",
		"ghas":       "ghas",
		"ghec":       "ghec",
	}

	// SKU-level identifiers (SkuPricing).
	skuLevel := map[string]string{
		// Copilot
		"copilot_premium_request":       "copilot_premium_request",
		"copilot_agent_premium_request": "copilot_agent_premium_request",
		"copilot_enterprise":            "copilot_enterprise",
		"copilot_for_business":          "copilot_for_business",
		"copilot_standalone":            "copilot_standalone",
		// Actions
		"actions_linux":   "actions_linux",
		"actions_macos":   "actions_macos",
		"actions_windows": "actions_windows",
		"actions_storage": "actions_storage",
		// Codespaces
		"codespaces_storage":          "codespaces_storage",
		"codespaces_prebuild_storage": "codespaces_prebuild_storage",
		// Packages
		"packages_storage":   "packages_storage",
		"packages_bandwidth": "packages_bandwidth",
		// GHAS
		"ghas_licenses":                   "ghas_licenses",
		"ghas_code_security_licenses":     "ghas_code_security_licenses",
		"ghas_secret_protection_licenses": "ghas_secret_protection_licenses",
		// Other
		"ghec_licenses":         "ghec_licenses",
		"git_lfs_storage":       "git_lfs_storage",
		"git_lfs_bandwidth":     "git_lfs_bandwidth",
		"models_inference":      "models_inference",
		"spark_premium_request": "spark_premium_request",
	}

	if sku, ok := skuLevel[p]; ok {
		return "SkuPricing", sku
	}
	if prod, ok := productLevel[p]; ok {
		return "ProductPricing", prod
	}

	// Unknown — default to SkuPricing.
	return "SkuPricing", p
}
