package onboarding

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/sourcerepo/v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"maintainerd/model"
	"net/http"
	"os"

	"github.com/google/go-github/v55/github"

	"maintainerd/db"
	"maintainerd/plugins/fossa"
)

// EventListener server that handles GitHub webhook events and triggers onboarding processes using the maintainerd db and
// known services such as FOSSA.
type EventListener struct {
	Store        *db.SQLStore
	FossaClient  *fossa.Client
	Secret       []byte
	Projects     map[string]model.Project
	Repo         sourcerepo.Repo
	GitHubClient *github.Client
}

func (s *EventListener) Init(dbPath, fossaAPItokenEnvVar, ghToken, repo, org string) error {
	dbConn, err := gorm.Open(sqlite.Open(dbPath))
	if err != nil {
		log.Printf("error: failed to connect to db: %v", err)
		return fmt.Errorf("connect to db: %w", err)
	}
	s.Store = db.NewSQLStore(dbConn)

	projectMap, err := s.Store.GetProjectMapByName()
	if err != nil {
		log.Printf("error: failed to get project map: %v", err)
		return fmt.Errorf("get project map: %w", err)
	}
	s.Projects = projectMap
	log.Printf("Init: DBG, project map has %d entries", len(s.Projects))
	log.Printf("Init: DBG, they are...")
	for _, project := range s.Projects {
		log.Printf("info: project: %s (%d)", project.Name, project.ID)
	}
	token := os.Getenv(fossaAPItokenEnvVar)
	if token == "" {
		log.Printf("Init: ERR, the environment variable %s must be set", fossaAPItokenEnvVar)
		return fmt.Errorf("missing required environment variable: %s", fossaAPItokenEnvVar)
	}
	s.FossaClient = fossa.NewClient(token)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ghToken})
	tc := oauth2.NewClient(context.Background(), ts)
	s.GitHubClient = github.NewClient(tc)

	log.Printf("info: EventListener initialized successfully for org %q and repo %q", org, repo)
	return nil
}

// Run starts an HTTP server listening on the given address.
func (s *EventListener) Run(addr string) error {
	http.HandleFunc("/webhook", s.handleWebhook)
	return http.ListenAndServe(addr, nil)
}

func (s *EventListener) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, s.Secret)
	if err != nil {
		http.Error(w, "handleWebhook: github.ValidatePayload, invalid signature", http.StatusUnauthorized)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "handleWebhook: could not parse event", http.StatusBadRequest)
		return
	}

	switch e := event.(type) {
	case *github.IssuesEvent:
		if e.GetAction() != "labeled" {
			break
		}
		for _, label := range e.Issue.Labels {
			name := label.GetName()
			if name == "fossa" {
				projectName, err := GetProjectNameFromProjectTitle(e.Issue.GetTitle())
				if err != nil {
					log.Printf("handleWebhook: WRN, failed to parse project name: %v", err)
					continue
				}
				log.Printf("handleWebhook: DBG, project name: %s", projectName)

				// Get Project from db
				var project model.Project
				project = s.Projects[projectName]
				actions, err := signProjectUpForFOSSA(r.Context(), s.Store, s.FossaClient, project)
				if err != nil {
					log.Printf("handleWebhook: ERR, failed to send FOSSA invitations: %v", err)
				}

				// Format the steps as a Markdown comment
				var comment string
				comment += "###  🧪 maintainerd - CNCF FOSSA Onboarding Report\n\n" +
					"#### :spiral_notepad: Actions taken during onboarding...\n\n"
				for _, action := range actions {
					comment += fmt.Sprintf("- %s\n", action)
				}
				if err != nil {
					comment += fmt.Sprintf("\n❌ Onboarding encountered some problems: `%s`\n", err)
				} else {
					comment += "---\n\n" +
						"Once accepted:\n\n" +
						"- 👤 The CNCF Projects Team *must first* add you to the " + projectName + " team as a **Team Admin** ([FOSSA RBAC](https://docs.fossa.com/docs/role-based-access-control#team-roles)).\n\n" +
						"- 📦 Then, _and only then_, can you start importing your code and documentation repositories into FOSSA: [Getting Started Guide](https://docs.fossa.com/docs/getting-started#importing-a-project).\n\n"
				}
				err = s.updateIssue(r.Context(), e.GetRepo().GetOwner().GetLogin(), e.GetRepo().GetName(), e.GetIssue().GetNumber(), comment)
				if err != nil {
					log.Printf("handleWebhook: WRN, failed to update GitHub issue: %v", err)
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

// signProjectUpForFOSSA using @store, gets the maintainers registered for @project, uses @fc to email them FOSSA invites
// to their registered email addresses. As invitations are sent, we build up a list of actions that were taken by the
// process so that the client can report steps taken and their results; in actions we reference maintainers using their
// public GitHub account keeping their registered email addresses private.
func signProjectUpForFOSSA(context context.Context, store *db.SQLStore, fc *fossa.Client, project model.Project) ([]string, error) {
	var actions []string

	// Check for maintainers registered for this project
	maintainers, err := store.GetMaintainersByProject(project.ID)
	if err != nil {
		actions = append(actions, fmt.Sprintf(":x: %s maintainers are not yet registered.", project.Name))
		return actions, fmt.Errorf("signProjectUpForFOSSA: no maintainers found for project %v, project ID", project)
	}

	actions = append(actions, fmt.Sprintf("✅  %s has %d registered maintainers", project.Name, len(maintainers)))

	// Do we have a team already in FOSSA for @project?
	serviceTeams, err := store.GetProjectServiceTeamMap("FOSSA")
	if err != nil {
		actions = append(actions, fmt.Sprintf(":warning: Problem retrieving serviceTeams.  %v", err))
	}
	st, ok := serviceTeams[project.ID]
	if ok {
		actions = append(
			actions,
			fmt.Sprintf("👥 [%s team](https://app.fossa.com/account/settings/organization/teams/%d) was already in FOSSA",
				project.Name,
				st.ServiceTeamID))
	} else {
		// create the team on FOSSA, add the team to the ServiceTeams
		team, err := fc.CreateTeam(project.Name)

		if err != nil {
			actions = append(actions, fmt.Sprintf(":x: Problem creating team on FOSSA for %s: %v", project.Name, err))
		} else {
			log.Printf("team created: %s", team.Name)
			actions = append(actions,
				fmt.Sprintf("👥  [%s team](https://app.fossa.com/account/settings/organization/teams/%d) has been created in FOSSA",
					team.Name, team.ID))
			_, err := store.CreateServiceTeam(project.ID, project.Name, team.ID, team.Name)
			if err != nil {
				fmt.Printf("handleWebhook: WRN, failed to create service team: %v", err)
			}
		}
		if err != nil {
			fmt.Printf("signProjectUpForFOSSA: Error creating team on FOSSA for %s: %v", project.Name, err)
		}
	}
	if len(maintainers) == 0 {
		actions = append(actions, fmt.Sprintf("Maintainers not yet registered, for project %s", project.Name))
		return actions, fmt.Errorf(":x: no maintainers found for project %d", project.ID)
	}
	for _, maintainer := range maintainers {
		err := fc.SendUserInvitation(maintainer.Email) // TODO See if I can Name the User on FOSSA!

		if errors.Is(err, fossa.ErrInviteAlreadyExists) {
			actions = append(actions, fmt.Sprintf("@%s : you have a pending invitation to join CNCF FOSSA. Please check your registered email and accept the invitation within 48 hours.", maintainer.GitHubAccount))
		} else if errors.Is(err, fossa.ErrUserAlreadyMember) {
			// TODO Edge case - maintainers who are already signed up in FOSSA on another project.
			log.Printf("user is already a member, skipping")
		} else if err != nil {
			log.Printf("error sending invite: %v", err)
			actions = append(actions, fmt.Sprintf("@%s : there was a problem sending a CNCF FOSSA invitation to you.", maintainer.GitHubAccount))
		}
	}

	return actions, nil
}

func (s *EventListener) updateIssue(ctx context.Context, owner, repo string, issueNumber int, comment string) error {
	issueComment := &github.IssueComment{
		Body: github.String(comment),
	}
	_, _, err := s.GitHubClient.Issues.CreateComment(ctx, owner, repo, issueNumber, issueComment)
	if err != nil {
		log.Printf("updateIssue: ERR, error creating comment: %v", err)
	}
	return err
}
