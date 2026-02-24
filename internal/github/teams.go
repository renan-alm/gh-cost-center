package github

import (
	"fmt"
	"net/http"
)

// Team represents a GitHub team (organization or enterprise level).
type Team struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// TeamMember represents a member of a GitHub team.
type TeamMember struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

// GetOrgTeams returns all teams for the given organization, handling
// pagination automatically.
func (c *Client) GetOrgTeams(org string) ([]Team, error) {
	c.log.Info("Fetching teams for organization", "org", org)
	baseURL := fmt.Sprintf("%s/orgs/%s/teams", c.baseURL, org)

	var allTeams []Team
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, perPage)
		var teams []Team
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &teams); err != nil {
			return nil, fmt.Errorf("fetching teams for org %s page %d: %w", org, page, err)
		}
		if len(teams) == 0 {
			break
		}
		allTeams = append(allTeams, teams...)
		c.log.Debug("Fetched teams page", "org", org, "page", page, "count", len(teams))
		if len(teams) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total teams found", "org", org, "count", len(allTeams))
	return allTeams, nil
}

// GetOrgTeamMembers returns all members of the specified organization team,
// handling pagination automatically.
func (c *Client) GetOrgTeamMembers(org, teamSlug string) ([]TeamMember, error) {
	c.log.Debug("Fetching members for team", "org", org, "team", teamSlug)
	baseURL := fmt.Sprintf("%s/orgs/%s/teams/%s/members", c.baseURL, org, teamSlug)

	var allMembers []TeamMember
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, perPage)
		var members []TeamMember
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &members); err != nil {
			return nil, fmt.Errorf("fetching members for team %s/%s page %d: %w", org, teamSlug, page, err)
		}
		if len(members) == 0 {
			break
		}
		allMembers = append(allMembers, members...)
		c.log.Debug("Fetched team members page",
			"org", org, "team", teamSlug, "page", page, "count", len(members))
		if len(members) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total members found", "team", org+"/"+teamSlug, "count", len(allMembers))
	return allMembers, nil
}

// GetEnterpriseTeams returns all teams in the enterprise, handling pagination
// automatically.
func (c *Client) GetEnterpriseTeams() ([]Team, error) {
	c.log.Info("Fetching enterprise teams", "enterprise", c.enterprise)
	baseURL := c.enterpriseURL("/teams")

	var allTeams []Team
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, perPage)
		var teams []Team
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &teams); err != nil {
			return nil, fmt.Errorf("fetching enterprise teams page %d: %w", page, err)
		}
		if len(teams) == 0 {
			break
		}
		allTeams = append(allTeams, teams...)
		c.log.Debug("Fetched enterprise teams page", "page", page, "count", len(teams))
		if len(teams) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total enterprise teams found", "count", len(allTeams))
	return allTeams, nil
}

// GetEnterpriseTeamMembers returns all members of the specified enterprise
// team, handling pagination automatically.
func (c *Client) GetEnterpriseTeamMembers(teamSlug string) ([]TeamMember, error) {
	c.log.Debug("Fetching members for enterprise team", "team", teamSlug)
	baseURL := c.enterpriseURL(fmt.Sprintf("/teams/%s/memberships", teamSlug))

	var allMembers []TeamMember
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", baseURL, page, perPage)
		var members []TeamMember
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &members); err != nil {
			return nil, fmt.Errorf("fetching enterprise team %s members page %d: %w", teamSlug, page, err)
		}
		if len(members) == 0 {
			break
		}
		allMembers = append(allMembers, members...)
		c.log.Debug("Fetched enterprise team members page",
			"team", teamSlug, "page", page, "count", len(members))
		if len(members) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total members found for enterprise team", "team", teamSlug, "count", len(allMembers))
	return allMembers, nil
}
