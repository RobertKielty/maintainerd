package fossa

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"time"
)

const (
	apiBase = "https://app.fossa.com/api"
)

type Client struct {
	APIKey  string
	APIBase string
}

func NewClient(token string) *Client {
	return &Client{
		APIKey:  token,
		APIBase: apiBase,
	}
}

// FetchUsers returns an array of User or an error
func (c *Client) FetchFirstPageOfUsers() ([]User, error) {
	req, _ := http.NewRequest("GET", c.APIBase+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchUsers failed: called $s\n\t\t%s – %s", resp.Status, string(body))
	}
	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}
func (c *Client) FetchUsers() ([]User, error) {
	var allUsers []User
	page := 1
	count := 100 // Adjust this value as per FOSSA API limits
	fmt.Printf("")
	for {
		// Construct paginated URL
		url := fmt.Sprintf("%s/users?count=%d&page=%d", c.APIBase, count, page)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		// Read body early for error handling/logging
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("FetchUsers failed: %s\n\t\t%s", resp.Status, string(body))
		}

		var users []User
		if err := json.Unmarshal(body, &users); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allUsers = append(allUsers, users...)

		// If we got fewer users than count, we’re done
		if len(users) < count {
			break
		}
		page++
	}

	return allUsers, nil
}

// FetchUserInvitations GETs /api/user-invitations - Retrieves all active (non-expired) user invitations for an
// organization
func (c *Client) FetchUserInvitations() (string, error) {
	req, _ := http.NewRequest("GET", c.APIBase+"/user-invitations", nil)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("FetchUserInvitations failed %s\n", err), err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("FetchUserInvitations failed: called $s\n\t\t%s – %s", resp.Status, string(body))
	}

	return string(body), nil
}

// FetchTeams calls GET /api/teams
func (c *Client) FetchTeams() ([]Team, error) {
	req, _ := http.NewRequest("GET", c.APIBase+"/teams", nil)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list teams failed: %s – %s", resp.Status, string(body))
	}

	var teams []Team
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// FetchTeamUserEmails calls GET /api/teams/{id}/members
func (c *Client) FetchTeamUserEmails(teamID int) ([]string, error) {
	var url = fmt.Sprintf("%s/teams/%d/members", c.APIBase, teamID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing response body: %v\n", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list team users failed: %s – %s", resp.Status, string(body))
	}
	var emails []string
	var members TeamMembers
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return nil, fmt.Errorf("list team users failed json.NewDecoder returned: %s\nwhen trying to decode %s", err, resp.Body)
	}
	if members.TotalCount > 0 {
		for _, result := range members.Results {
			emails = append(emails, result.Email)
		}
	}
	return emails, nil
}

// GetTeamId searches a slice of Team objects by name.
// Returns the team’s ID if found, or an error “team not found” if not.
func (c *Client) GetTeamId(teams []Team, name string) (int, error) {
	for _, t := range teams {
		if t.Name == name {
			return t.ID, nil
		}
	}
	return 0, fmt.Errorf("team not found: %q", name)
}

// FetchTeamsMap returns a map of FOSSA Teams keyed by the name of the team
func (c *Client) FetchTeamsMap() (map[string]Team, error) {
	ta, err := c.FetchTeams()
	if err != nil {
		log.Printf("FOSSA client, FetchTeamsMap:Error fetching teams: %v", err)
		return nil, err
	}
	tm := map[string]Team{}

	for i, team := range ta {
		tm[team.Name] = ta[i]
	}
	return tm, nil
}

type TeamMembers struct {
	Results []struct {
		UserID   int    `json:"userId"`
		RoleID   int    `json:"roleId"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"results"`
	PageSize   int `json:"pageSize"`
	Page       int `json:"page"`
	TotalCount int `json:"totalCount"`
}

// Team models a single team object from GET /api/teams
type Team struct {
	ID               int       `json:"id"`
	OrganizationID   int       `json:"organizationId"`
	Name             string    `json:"name"`
	DefaultRoleID    int       `json:"defaultRoleId"`
	AutoAddUsers     bool      `json:"autoAddUsers"`
	UniqueIdentifier string    `json:"uniqueIdentifier"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	TeamUsers        []struct {
		UserID int `json:"userId"`
		RoleID int `json:"roleId"`
	} `json:"teamUsers"`
	TeamReleaseGroupsCount int `json:"teamReleaseGroupsCount"`
	TeamProjectsCount      int `json:"teamProjectsCount"`
}

// User models the JSON returned by GET /api/users/{id}
type User struct {
	ID             int         `json:"id"`
	Username       string      `json:"username"`
	Email          string      `json:"email"`
	EmailVerified  bool        `json:"email_verified"`
	Demo           bool        `json:"demo"`
	Super          bool        `json:"super"`
	Joined         time.Time   `json:"joined"`
	LastVisit      time.Time   `json:"last_visit"`
	TermsAgreed    *time.Time  `json:"terms_agreed"`
	FullName       string      `json:"full_name"`
	Phone          string      `json:"phone"`
	Role           string      `json:"role"`
	OrganizationID int         `json:"organizationId"`
	SSOOnly        bool        `json:"sso_only"`
	Enabled        bool        `json:"enabled"`
	HasSetPassword *bool       `json:"has_set_password"`
	InstallAdmin   *bool       `json:"install_admin"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	UserRole       interface{} `json:"userRole"`
	Tokens         []struct {
		ID         int       `json:"id"`
		Name       string    `json:"name"`
		IsDisabled bool      `json:"isDisabled"`
		UpdatedAt  time.Time `json:"updatedAt"`
		CreatedAt  time.Time `json:"createdAt"`
		Meta       struct {
			PushOnly bool `json:"pushOnly"`
		} `json:"meta"`
	} `json:"tokens"`
	GitHub struct {
		Name      *string `json:"name"`
		Email     *string `json:"email"`
		AvatarURL string  `json:"avatar_url"`
	} `json:"github"`
	TeamUsers []struct {
		RoleID int `json:"roleId"`
		Team   struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
	} `json:"teamUsers"`
	Organization struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		AccessLevel string `json:"access_level"`
	} `json:"organization"`
}
