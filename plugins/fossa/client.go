package fossa

import (
	"encoding/json"
	"fmt"
	"io"
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

// FetchTeamUsers calls GET /api/teams/{teamID}/users
func (c *Client) FetchTeamUsers(teamID int) ([]User, error) {
	var url = fmt.Sprintf("%s/teams/%d/users", c.APIBase, teamID)
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

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
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

// FetchTeamUserEmails returns an array of email addresses for members of the team identified by teamId
func (c *Client) FetchTeamUserEmails(teamID int) ([]string, error) {
	teams, err := c.FetchTeams()
	if err != nil {
		return nil, fmt.Errorf("fetchTeams: %w", err)
	}

	var userIDs []int
	for _, t := range teams {
		if t.ID == teamID {
			for _, tu := range t.TeamUsers {
				userIDs = append(userIDs, tu.UserID)
			}
			break
		}
	}
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("no users found for team %d", teamID)
	}
	var emails []string
	for _, uid := range userIDs {
		url := fmt.Sprintf("%s/users/%d", c.APIBase, uid)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request /users/%d failed: %w", uid, err)
		}
		defer func(Body io.ReadCloser) {
			if err := Body.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing response body: %v\n", err)
			}
		}(resp.Body)

		var users []User
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			return nil, err
		}
		for _, user := range users {
			emails = append(emails, user.Email)
		}

	}
	return emails, nil
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
