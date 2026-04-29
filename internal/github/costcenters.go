package github

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// costCentersListResponse is the JSON envelope for the list endpoint.
type costCentersListResponse struct {
	CostCenters []CostCenter `json:"costCenters"`
}

// CostCenter represents a billing cost center returned by the API.
type CostCenter struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"` // "active", "deleted", etc.
}

// costCenterCreateResponse is the JSON envelope for the create endpoint.
type costCenterCreateResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// costCenterDetailResponse is the JSON envelope for the detail endpoint.
type costCenterDetailResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	Resources []Resource `json:"resources"`
}

// Resource represents a user or repository assigned to a cost center.
type Resource struct {
	Type string `json:"type"` // "User", "Repository", etc.
	Name string `json:"name"`
}

// membershipResponse is the JSON envelope for the memberships endpoint.
type membershipResponse struct {
	Memberships []Membership `json:"memberships"`
}

// Membership describes a user's cost center membership.
type Membership struct {
	CostCenter CostCenterRef `json:"cost_center"`
}

// CostCenterRef is a lightweight cost center reference within a membership.
type CostCenterRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// uuidFromConflictRe extracts a UUID from the 409 conflict error message body.
var uuidFromConflictRe = regexp.MustCompile(
	`(?i)existing cost center UUID:\s*([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`,
)

// uuidRe matches a standard UUID format.
var uuidRe = regexp.MustCompile(
	`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`,
)

// IsCostCenterNotFound returns true if the error is a 404 API error
// for a cost center lookup.
func IsCostCenterNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// IsValidCostCenterUUID returns true if the string is a valid UUID format.
func IsValidCostCenterUUID(id string) bool {
	return uuidRe.MatchString(id)
}

// ValidateCostCenterID returns an error if the given ID is not a valid UUID.
// This catches the common misconfiguration where a cost center name is used
// in place of a UUID.
func ValidateCostCenterID(id string) error {
	if IsValidCostCenterUUID(id) {
		return nil
	}
	for _, r := range id {
		if r > 127 {
			return fmt.Errorf(
				"cost center ID %q is not a valid UUID and contains non-ASCII characters — "+
					"this looks like a cost center name, not an ID; enable auto_create_cost_centers "+
					"or use the UUID from enterprise billing settings",
				id)
		}
	}
	return fmt.Errorf(
		"cost center ID %q is not a valid UUID — "+
			"this may be a cost center name instead of an ID; enable auto_create_cost_centers "+
			"or use the UUID from enterprise billing settings",
		id)
}

// GetAllActiveCostCenters returns a map of cost center name → ID for all
// active cost centers in the enterprise.
func (c *Client) GetAllActiveCostCenters() (map[string]string, error) {
	url := c.enterpriseURL("/settings/billing/cost-centers")

	var resp costCentersListResponse
	if _, err := c.doJSON(http.MethodGet, url, nil, &resp); err != nil {
		return nil, fmt.Errorf("fetching cost centers: %w", err)
	}

	active := make(map[string]string)
	for _, cc := range resp.CostCenters {
		if cc.State == "active" && cc.Name != "" && cc.ID != "" {
			active[cc.Name] = cc.ID
			// Populate cache with every active cost center.
			if c.ccCache != nil {
				_ = c.ccCache.Set(cc.Name, cc.ID, cc.Name)
			}
		}
	}
	c.log.Debug("Found active cost centers", "active", len(active), "total", len(resp.CostCenters))
	return active, nil
}

// GetCostCenter returns the details of a single cost center including its
// assigned resources.
func (c *Client) GetCostCenter(id string) (*costCenterDetailResponse, error) {
	if err := ValidateCostCenterID(id); err != nil {
		return nil, err
	}
	url := c.enterpriseURL(fmt.Sprintf("/settings/billing/cost-centers/%s", id))
	var resp costCenterDetailResponse
	if _, err := c.doJSON(http.MethodGet, url, nil, &resp); err != nil {
		return nil, fmt.Errorf("fetching cost center %s: %w", id, err)
	}
	return &resp, nil
}

// GetCostCenterMembers returns the usernames of all users assigned to the
// given cost center.
func (c *Client) GetCostCenterMembers(id string) ([]string, error) {
	detail, err := c.GetCostCenter(id)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, r := range detail.Resources {
		if r.Type == "User" && r.Name != "" {
			users = append(users, r.Name)
		}
	}
	c.log.Debug("Cost center members", "cost_center_id", id, "count", len(users))
	return users, nil
}

// CreateCostCenter creates a new cost center with the given name.  If the cost
// center already exists (409 Conflict) it attempts to extract the existing UUID
// from the error message.  If that fails it falls back to searching by name.
func (c *Client) CreateCostCenter(name string) (string, error) {
	// Check cache first.
	if c.ccCache != nil {
		if entry, ok := c.ccCache.Get(name); ok {
			c.log.Debug("Cost center found in cache", "name", name, "id", entry.ID)
			return entry.ID, nil
		}
	}

	url := c.enterpriseURL("/settings/billing/cost-centers")
	body := map[string]string{"name": name}

	var resp costCenterCreateResponse
	_, err := c.doJSON(http.MethodPost, url, body, &resp)
	if err == nil {
		c.log.Info("Created cost center", "name", name, "id", resp.ID)
		// Update cache with newly created cost center.
		if c.ccCache != nil {
			_ = c.ccCache.Set(name, resp.ID, name)
		}
		return resp.ID, nil
	}

	// Handle 409 Conflict — cost center already exists.
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		c.log.Info("Cost center already exists, extracting existing ID", "name", name)

		if m := uuidFromConflictRe.FindStringSubmatch(apiErr.Body); len(m) == 2 {
			c.log.Info("Extracted existing cost center ID from API response", "id", m[1])
			// Update cache with extracted ID.
			if c.ccCache != nil {
				_ = c.ccCache.Set(name, m[1], name)
			}
			return m[1], nil
		}

		c.log.Warn("Could not extract UUID from 409 response, falling back to name search", "name", name)
		return c.findCostCenterByName(name)
	}

	return "", fmt.Errorf("creating cost center %q: %w", name, err)
}

// CreateCostCenterWithPreload creates a cost center with preload optimization.
// If the name already exists in the given map, it returns the cached ID.
// On successful creation (or 409 extraction), it updates the map.
func (c *Client) CreateCostCenterWithPreload(name string, activeMap map[string]string) (string, error) {
	if id, ok := activeMap[name]; ok {
		c.log.Debug("Found cost center in preload map", "name", name, "id", id)
		return id, nil
	}

	// Check file-based cache before making API call.
	if c.ccCache != nil {
		if entry, ok := c.ccCache.Get(name); ok {
			c.log.Debug("Found cost center in cache", "name", name, "id", entry.ID)
			activeMap[name] = entry.ID
			return entry.ID, nil
		}
	}

	id, err := c.CreateCostCenter(name)
	if err != nil {
		return "", err
	}
	activeMap[name] = id
	return id, nil
}

// findCostCenterByName searches the list of all cost centers for an active one
// with the exact name.
func (c *Client) findCostCenterByName(name string) (string, error) {
	active, err := c.GetAllActiveCostCenters()
	if err != nil {
		return "", fmt.Errorf("finding cost center by name %q: %w", name, err)
	}
	if id, ok := active[name]; ok {
		c.log.Info("Found active cost center by name", "name", name, "id", id)
		return id, nil
	}
	return "", fmt.Errorf("no active cost center found with name %q", name)
}

// EnsureCostCentersExist creates (or retrieves) the two PRU-tier cost centers,
// returning their IDs.
func (c *Client) EnsureCostCentersExist(noPRUName, pruAllowedName string) (noPRUID, pruAllowedID string, err error) {
	c.log.Info("Ensuring cost center exists", "name", noPRUName)
	noPRUID, err = c.CreateCostCenter(noPRUName)
	if err != nil {
		return "", "", fmt.Errorf("ensuring cost center %q: %w", noPRUName, err)
	}

	c.log.Info("Ensuring cost center exists", "name", pruAllowedName)
	pruAllowedID, err = c.CreateCostCenter(pruAllowedName)
	if err != nil {
		return "", "", fmt.Errorf("ensuring cost center %q: %w", pruAllowedName, err)
	}

	c.log.Info("Cost centers ready", "no_pru_id", noPRUID, "pru_allowed_id", pruAllowedID)
	return noPRUID, pruAllowedID, nil
}

// ResolveCostCenters resolves two cost center names to UUIDs without creating
// them.  Returns an error listing any names that could not be found.
func (c *Client) ResolveCostCenters(noPRUName, pruAllowedName string) (noPRUID, pruAllowedID string, err error) {
	c.log.Info("Resolving cost center names to IDs (no creation)")

	activeMap, err := c.GetAllActiveCostCenters()
	if err != nil {
		return "", "", fmt.Errorf("fetching active cost centers for resolution: %w", err)
	}

	var unresolved []string

	noPRUID, ok := activeMap[noPRUName]
	if !ok {
		unresolved = append(unresolved, noPRUName)
	}

	pruAllowedID, ok = activeMap[pruAllowedName]
	if !ok {
		unresolved = append(unresolved, pruAllowedName)
	}

	if len(unresolved) > 0 {
		return "", "", fmt.Errorf(
			"cost center(s) not found: %s — verify the names match exactly as they appear "+
				"in GitHub Enterprise billing settings, or use --create-cost-centers to create them automatically",
			strings.Join(unresolved, ", "),
		)
	}

	c.log.Info("Cost centers resolved", "no_pru_id", noPRUID, "pru_allowed_id", pruAllowedID)
	return noPRUID, pruAllowedID, nil
}

// AddUsersToCostCenter adds a batch of usernames to a cost center.  The GitHub
// API allows a maximum of 50 users per request, so this method handles chunking
// transparently.
//
// When ignoreCurrentCC is false, users already assigned to another cost center
// are skipped.  When true, users are added regardless of existing membership.
//
// Returns a map of username → success status.
func (c *Client) AddUsersToCostCenter(costCenterID string, usernames []string, ignoreCurrentCC bool) (map[string]bool, error) {
	if len(usernames) == 0 {
		return map[string]bool{}, nil
	}

	if err := ValidateCostCenterID(costCenterID); err != nil {
		return nil, err
	}

	results := make(map[string]bool, len(usernames))

	// Check which users are already in the target cost center.
	currentMembers, err := c.GetCostCenterMembers(costCenterID)
	if err != nil {
		if IsCostCenterNotFound(err) {
			return nil, fmt.Errorf(
				"cost center ID %q not found — verify the cost center exists in enterprise billing settings, "+
					"or enable auto_create_cost_centers to create it automatically: %w",
				costCenterID, err)
		}
		return nil, fmt.Errorf("checking cost center members: %w", err)
	}
	memberSet := toSet(currentMembers)

	var toAdd []string
	for _, u := range usernames {
		if memberSet[u] {
			results[u] = true // already in target
			continue
		}

		if !ignoreCurrentCC {
			mem, _ := c.CheckUserCostCenterMembership(u)
			if mem != nil {
				c.log.Info("Skipping user already in another cost center",
					"user", u, "current_cost_center", mem.Name)
				results[u] = false
				continue
			}
		}
		toAdd = append(toAdd, u)
	}

	if len(toAdd) == 0 {
		c.log.Info("All users already assigned", "cost_center_id", costCenterID)
		return results, nil
	}

	c.log.Info("Adding users to cost center",
		"cost_center_id", costCenterID,
		"to_add", len(toAdd),
		"already_assigned", len(usernames)-len(toAdd),
	)

	// Chunk into batches of 50.
	const batchSize = 50
	for i := 0; i < len(toAdd); i += batchSize {
		end := i + batchSize
		if end > len(toAdd) {
			end = len(toAdd)
		}
		batch := toAdd[i:end]

		url := c.enterpriseURL(fmt.Sprintf("/settings/billing/cost-centers/%s/resource", costCenterID))
		body := map[string]any{"users": batch}

		_, err := c.doJSON(http.MethodPost, url, body, nil)
		if err != nil {
			c.log.Error("Failed to add users batch", "cost_center_id", costCenterID, "batch_size", len(batch), "error", err)
			for _, u := range batch {
				results[u] = false
			}
			continue
		}
		c.log.Info("Successfully added users batch", "cost_center_id", costCenterID, "batch_size", len(batch))
		for _, u := range batch {
			results[u] = true
		}
	}

	return results, nil
}

// BulkUpdateCostCenterAssignments processes multiple cost center → usernames
// mappings, chunking and deduplicating as needed.
func (c *Client) BulkUpdateCostCenterAssignments(assignments map[string][]string, ignoreCurrentCC bool) (map[string]map[string]bool, error) {
	results := make(map[string]map[string]bool)
	totalUsers := 0
	successUsers := 0
	failedUsers := 0

	for ccID, usernames := range assignments {
		if len(usernames) == 0 {
			continue
		}
		totalUsers += len(usernames)

		ccResults, err := c.AddUsersToCostCenter(ccID, usernames, ignoreCurrentCC)
		if err != nil {
			if IsCostCenterNotFound(err) {
				c.log.Error("Cost center not found — this usually means a cost center name was used instead of a UUID",
					"cost_center_id", ccID,
					"hint", "enable auto_create_cost_centers or verify the ID in enterprise billing settings",
					"error", err)
			} else {
				c.log.Error("Failed to update cost center assignments", "cost_center_id", ccID, "error", err)
			}
			ccResults = make(map[string]bool, len(usernames))
			for _, u := range usernames {
				ccResults[u] = false
			}
		}
		results[ccID] = ccResults

		for _, ok := range ccResults {
			if ok {
				successUsers++
			} else {
				failedUsers++
			}
		}
	}

	c.log.Info("Assignment results", "successful", successUsers, "total", totalUsers)
	if failedUsers > 0 {
		c.log.Error("Some users failed assignment", "failed", failedUsers)
	}
	return results, nil
}

// RemoveUsersFromCostCenter removes a list of usernames from a cost center.
func (c *Client) RemoveUsersFromCostCenter(costCenterID string, usernames []string) (map[string]bool, error) {
	if len(usernames) == 0 {
		return map[string]bool{}, nil
	}

	if err := ValidateCostCenterID(costCenterID); err != nil {
		return nil, err
	}

	url := c.enterpriseURL(fmt.Sprintf("/settings/billing/cost-centers/%s/resource", costCenterID))
	body := map[string]any{"users": usernames}

	_, err := c.doJSON(http.MethodDelete, url, body, nil)
	if err != nil {
		c.log.Error("Failed to remove users from cost center",
			"cost_center_id", costCenterID, "error", err)
		result := make(map[string]bool, len(usernames))
		for _, u := range usernames {
			result[u] = false
		}
		return result, fmt.Errorf("removing users from cost center %s: %w", costCenterID, err)
	}

	c.log.Info("Successfully removed users from cost center",
		"cost_center_id", costCenterID, "count", len(usernames))
	result := make(map[string]bool, len(usernames))
	for _, u := range usernames {
		result[u] = true
	}
	return result, nil
}

// CheckUserCostCenterMembership checks whether a user belongs to any cost
// center.  Returns the cost center reference if found, nil otherwise.
func (c *Client) CheckUserCostCenterMembership(username string) (*CostCenterRef, error) {
	url := c.enterpriseURL(fmt.Sprintf(
		"/settings/billing/cost-centers/memberships?resource_type=user&name=%s", username,
	))

	var resp membershipResponse
	if _, err := c.doJSON(http.MethodGet, url, nil, &resp); err != nil {
		c.log.Debug("Failed to check cost center membership", "user", username, "error", err)
		return nil, nil // treat lookup failures as "not in any cost center"
	}

	if len(resp.Memberships) > 0 {
		ref := &resp.Memberships[0].CostCenter
		c.log.Debug("User belongs to cost center", "user", username, "cost_center_id", ref.ID)
		return ref, nil
	}
	c.log.Debug("User not in any cost center", "user", username)
	return nil, nil
}

// AddRepositoriesToCostCenter adds repository full-names (org/repo) to a cost
// center.
func (c *Client) AddRepositoriesToCostCenter(costCenterID string, repoNames []string) error {
	if len(repoNames) == 0 {
		return nil
	}

	c.log.Info("Adding repositories to cost center",
		"cost_center_id", costCenterID, "count", len(repoNames))

	url := c.enterpriseURL(fmt.Sprintf("/settings/billing/cost-centers/%s/resource", costCenterID))
	body := map[string]any{"repositories": repoNames}

	_, err := c.doJSON(http.MethodPost, url, body, nil)
	if err != nil {
		return fmt.Errorf("adding repositories to cost center %s: %w", costCenterID, err)
	}

	c.log.Info("Successfully added repositories to cost center",
		"cost_center_id", costCenterID, "count", len(repoNames))
	return nil
}

// RemoveRepositoriesFromCostCenter removes repository full-names (org/repo)
// from a cost center.
func (c *Client) RemoveRepositoriesFromCostCenter(costCenterID string, repoNames []string) error {
	if len(repoNames) == 0 {
		return nil
	}

	c.log.Info("Removing repositories from cost center",
		"cost_center_id", costCenterID, "count", len(repoNames))

	url := c.enterpriseURL(fmt.Sprintf("/settings/billing/cost-centers/%s/resource", costCenterID))
	body := map[string]any{"repositories": repoNames}

	_, err := c.doJSON(http.MethodDelete, url, body, nil)
	if err != nil {
		return fmt.Errorf("removing repositories from cost center %s: %w", costCenterID, err)
	}

	c.log.Info("Successfully removed repositories from cost center",
		"cost_center_id", costCenterID, "count", len(repoNames))
	return nil
}

// GetCostCenterRepos returns the repository names assigned to the given
// cost center.
func (c *Client) GetCostCenterRepos(id string) ([]string, error) {
	detail, err := c.GetCostCenter(id)
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, r := range detail.Resources {
		if r.Type == "Repository" && r.Name != "" {
			repos = append(repos, r.Name)
		}
	}
	c.log.Debug("Cost center repositories", "cost_center_id", id, "count", len(repos))
	return repos, nil
}

// toSet converts a string slice to a set (map[string]bool).
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
