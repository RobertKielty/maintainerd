package fossa_test

import (
	"os"
	"testing"

	"maintainerd/plugins/fossa"

	"github.com/stretchr/testify/assert"
)

const defaultE2ETeam = "SOPS"
const defaultImportedProjectCount = 5

func newE2EClient(t *testing.T) (*fossa.Client, string) {
	t.Helper()

	token := os.Getenv("FOSSA_API_TOKEN")
	if token == "" {
		t.Skip("FOSSA_API_TOKEN not set; skipping live FOSSA end-to-end tests")
	}

	client := fossa.NewClient(token)
	if apiBase := os.Getenv("FOSSA_API_BASE"); apiBase != "" {
		client.APIBase = apiBase
	}

	team := os.Getenv("FOSSA_TEST_TEAM")
	if team == "" {
		team = defaultE2ETeam
	}

	return client, team
}

func lookupTeamID(t *testing.T, client *fossa.Client, teamName string) uint {
	t.Helper()

	teams, err := client.FetchTeams()
	if err != nil {
		t.Fatalf("FetchTeams returned error: %v", err)
	}

	for _, team := range teams {
		if team.Name == teamName {
			return team.ID
		}
	}

	t.Fatalf("could not locate team ID for %q", teamName)
	return 0
}

func TestFetchUsersE2E(t *testing.T) {
	client, _ := newE2EClient(t)

	users, err := client.FetchUsers()
	if err != nil {
		t.Fatalf("FetchUsers returned error: %v", err)
	}
	if len(users) == 0 {
		t.Fatalf("FetchUsers returned zero users")
	}
}

func TestFetchTeamsE2E(t *testing.T) {
	client, teamName := newE2EClient(t)

	teams, err := client.FetchTeams()
	if err != nil {
		t.Fatalf("FetchTeams returned error: %v", err)
	}
	if len(teams) == 0 {
		t.Fatalf("FetchTeams returned zero teams")
	}

	var found bool
	for _, team := range teams {
		if team.Name == teamName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("FetchTeams did not include expected test team %q", teamName)
	}
}

func TestFetchTeamUserEmailsE2E(t *testing.T) {
	client, teamName := newE2EClient(t)

	targetTeamID := lookupTeamID(t, client, teamName)

	emails, err := client.FetchTeamUserEmails(targetTeamID)
	if err != nil {
		t.Fatalf("FetchTeamUserEmails returned error: %v", err)
	}
	if len(emails) == 0 {
		t.Fatalf("FetchTeamUserEmails returned zero emails for team %q", teamName)
	}
}

func TestFetchImportedReposE2E(t *testing.T) {
	client, teamName := newE2EClient(t)
	targetTeamID := lookupTeamID(t, client, teamName)

	count, repos, err := client.FetchImportedRepos(targetTeamID)
	if err != nil {
		t.Fatalf("FetchImportedRepos returned error: %v", err)
	}

	if count == 0 {
		t.Fatalf("FetchImportedRepos reported zero imported repositories for team %q", teamName)
	}
	if len(repos.Results) == 0 {
		t.Fatalf("FetchImportedRepos returned zero project entries for team %q", teamName)
	}
	if len(repos.Results) > count {
		t.Fatalf("FetchImportedRepos returned %d repo names, which exceeds count %d", len(repos.Results), count)
	}
	assert.Equal(t, defaultImportedProjectCount, len(repos.Results))
}
