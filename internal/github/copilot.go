package github

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// CopilotUser represents a Copilot seat holder returned by the billing/seats
// endpoint.
type CopilotUser struct {
	Login                   string `json:"login"`
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	Email                   string `json:"email"`
	Type                    string `json:"type"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
	PendingCancellationDate string `json:"pending_cancellation_date"`
	LastActivityAt          string `json:"last_activity_at"`
	LastActivityEditor      string `json:"last_activity_editor"`
	Plan                    string `json:"plan"`
	AssigningTeam           any    `json:"assigning_team"` // may be object or null
}

// seatsResponse is the JSON envelope returned by the Copilot billing seats API.
type seatsResponse struct {
	Seats      []seatEntry `json:"seats"`
	TotalSeats int         `json:"total_seats"`
}

type seatEntry struct {
	Assignee                assignee `json:"assignee"`
	CreatedAt               string   `json:"created_at"`
	UpdatedAt               string   `json:"updated_at"`
	PendingCancellationDate string   `json:"pending_cancellation_date"`
	LastActivityAt          string   `json:"last_activity_at"`
	LastActivityEditor      string   `json:"last_activity_editor"`
	Plan                    string   `json:"plan"`
	AssigningTeam           any      `json:"assigning_team"`
}

type assignee struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

// GetCopilotUsers returns all Copilot seat holders across the enterprise,
// handling pagination and deduplicating by login.
func (c *Client) GetCopilotUsers() ([]CopilotUser, error) {
	c.log.Info("Fetching Copilot users", "enterprise", c.enterprise)

	url := c.enterpriseURL("/copilot/billing/seats")
	var allUsers []CopilotUser
	page := 1
	const perPage = 100

	for {
		pageURL := fmt.Sprintf("%s?page=%d&per_page=%d", url, page, perPage)
		var resp seatsResponse
		if _, err := c.doJSON(http.MethodGet, pageURL, nil, &resp); err != nil {
			return nil, fmt.Errorf("fetching copilot seats page %d: %w", page, err)
		}

		if len(resp.Seats) == 0 {
			break
		}

		for _, s := range resp.Seats {
			allUsers = append(allUsers, CopilotUser{
				Login:                   s.Assignee.Login,
				ID:                      s.Assignee.ID,
				Name:                    s.Assignee.Name,
				Email:                   s.Assignee.Email,
				Type:                    s.Assignee.Type,
				CreatedAt:               s.CreatedAt,
				UpdatedAt:               s.UpdatedAt,
				PendingCancellationDate: s.PendingCancellationDate,
				LastActivityAt:          s.LastActivityAt,
				LastActivityEditor:      s.LastActivityEditor,
				Plan:                    s.Plan,
				AssigningTeam:           s.AssigningTeam,
			})
		}

		c.log.Debug("Fetched copilot seats page", "page", page, "count", len(resp.Seats))
		if len(resp.Seats) < perPage {
			break
		}
		page++
	}

	c.log.Info("Total Copilot users found", "count", len(allUsers))

	// Deduplicate by login.
	unique := deduplicateUsers(allUsers, c.log)
	return unique, nil
}

// deduplicateUsers removes duplicate entries, keeping the first occurrence of
// each login.
func deduplicateUsers(users []CopilotUser, logger *slog.Logger) []CopilotUser {
	seen := make(map[string]bool, len(users))
	dupCounts := make(map[string]int)
	unique := make([]CopilotUser, 0, len(users))

	for _, u := range users {
		if u.Login == "" {
			continue
		}
		if seen[u.Login] {
			dupCounts[u.Login]++
			continue
		}
		seen[u.Login] = true
		unique = append(unique, u)
	}

	if len(dupCounts) > 0 {
		total := 0
		for _, v := range dupCounts {
			total += v
		}
		logger.Warn("Detected and skipped duplicate seat entries",
			"duplicate_entries", total,
			"unique_users_affected", len(dupCounts),
		)
		logger.Info("Unique Copilot users after deduplication", "count", len(unique))
	}
	return unique
}

// FilterUsersByTimestamp returns only users whose created_at is strictly after
// the given threshold.  This is used for incremental processing — only new
// users since the last run are returned.
func FilterUsersByTimestamp(users []CopilotUser, after time.Time) []CopilotUser {
	var filtered []CopilotUser
	for _, u := range users {
		if u.CreatedAt == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, u.CreatedAt)
		if err != nil {
			// Try alternative format without timezone (some API responses).
			t, err = time.Parse("2006-01-02T15:04:05Z", u.CreatedAt)
			if err != nil {
				continue
			}
		}
		if t.After(after) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}
