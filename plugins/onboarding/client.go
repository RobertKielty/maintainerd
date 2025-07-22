package onboarding

import (
	"context"
	"log"
	"maintainerd/db"
	"maintainerd/plugins/fossa"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

// Client wraps a GitHub client and references to other subsystems such as the database.
type Client struct {
	gh          *github.Client
	fossaClient *fossa.Client
	store       db.Store
}

// NewClient creates a new Client using the provided GitHub access token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)

	return &Client{gh: github.NewClient(tc)}
}

// listIssues returns all open issues with the provided labels in the specified organization and repository.
func (c *Client) listIssues(ctx context.Context, owner, repo string, labels []string) ([]*github.Issue, error) {
	opts := &github.IssueListByRepoOptions{State: "open", Labels: labels}
	var all []*github.Issue
	for {
		issues, resp, err := c.gh.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// FetchSandboxOnboardingTasks lists tasks from issues labeled for onboarding in the sandbox repository.
func (c *Client) FetchSandboxOnboardingTasks(ctx context.Context) ([]Task, error) {
	owner := "cncf"
	repo := "sandbox"
	issues, err := c.listIssues(ctx, owner, repo, []string{"project onboarding", "sandbox"})
	if err != nil {
		return nil, err
	}

	var tasks []Task
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue
		}
		projectName, err := getProjectNameFromProjectTitle(issue.GetTitle())
		if err != nil {
			log.Printf("failed to parse project name for issue %d: %v", issue.GetNumber(), err)
			continue
		}
		tasks = append(tasks, getOnboardingTasks(projectName, issue.GetBody())...)
	}
	return tasks, nil
}
func (c *Client) SendFossaInvitations(context context.Context, store db.Store, client *fossa.Client, project string) interface{} {

}
