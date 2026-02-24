package github

import (
	"fmt"
	"net/http"
)

// RepoProperties represents a repository with its custom property values.
type RepoProperties struct {
	RepositoryID       int64      `json:"repository_id"`
	RepositoryName     string     `json:"repository_name"`
	RepositoryFullName string     `json:"repository_full_name"`
	Properties         []Property `json:"properties"`
}

// Property is a single custom property name-value pair.
type Property struct {
	PropertyName string `json:"property_name"`
	Value        any    `json:"value"` // can be string, []string, etc.
}

// PropertyDefinition is the schema definition for an organization custom
// property.
type PropertyDefinition struct {
	PropertyName  string   `json:"property_name"`
	ValueType     string   `json:"value_type"`
	Required      bool     `json:"required"`
	DefaultValue  any      `json:"default_value"`
	AllowedValues []string `json:"allowed_values"`
}

// GetOrgPropertySchema returns all custom property definitions for the given
// organization.
func (c *Client) GetOrgPropertySchema(org string) ([]PropertyDefinition, error) {
	c.log.Info("Fetching custom property schema", "org", org)
	url := fmt.Sprintf("%s/orgs/%s/properties/schema", c.baseURL, org)

	var defs []PropertyDefinition
	if _, err := c.doJSON(http.MethodGet, url, nil, &defs); err != nil {
		return nil, fmt.Errorf("fetching property schema for org %s: %w", org, err)
	}
	c.log.Info("Custom properties defined", "org", org, "count", len(defs))
	return defs, nil
}

// GetOrgReposWithProperties returns all repositories with their custom
// property values for the given organization, handling pagination.  An optional
// query string (GitHub search syntax) narrows the results.
func (c *Client) GetOrgReposWithProperties(org string, query string) ([]RepoProperties, error) {
	c.log.Info("Fetching repositories with custom properties", "org", org)
	baseURL := fmt.Sprintf("%s/orgs/%s/properties/values", c.baseURL, org)

	var allRepos []RepoProperties
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, perPage)
		if query != "" {
			pageURL += "&repository_query=" + query
		}

		var repos []RepoProperties
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &repos); err != nil {
			return nil, fmt.Errorf("fetching repos with properties for org %s page %d: %w", org, page, err)
		}
		if len(repos) == 0 {
			break
		}
		allRepos = append(allRepos, repos...)
		c.log.Debug("Fetched repos with properties page", "org", org, "page", page, "count", len(repos))
		if len(repos) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total repositories with custom properties", "org", org, "count", len(allRepos))
	return allRepos, nil
}

// GetRepoProperties returns custom property values for a specific repository.
func (c *Client) GetRepoProperties(owner, repo string) ([]Property, error) {
	c.log.Debug("Fetching custom properties for repository", "repo", owner+"/"+repo)
	url := fmt.Sprintf("%s/repos/%s/%s/properties/values", c.baseURL, owner, repo)

	var props []Property
	if _, err := c.doJSON(http.MethodGet, url, nil, &props); err != nil {
		return nil, fmt.Errorf("fetching properties for %s/%s: %w", owner, repo, err)
	}
	return props, nil
}
