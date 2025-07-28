package onboarding

import (
	"context"
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

// EventListener is a server that handles GitHub webhook events and triggers onboarding procedures
type EventListener struct {
	Store       *db.SQLStore
	FossaClient *fossa.Client
	Secret      []byte
	Projects    map[string]model.Project
}

func (s *EventListener) Init(dbPath, apiTokenEnvVar string) error {
	dbConn, err := gorm.Open(sqlite.Open(dbPath))
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	s.Store = db.NewSQLStore(dbConn)
	s.Projects, err = s.Store.GetProjectMapByName()
	if err != nil {
		log.Fatalf("failed to get project map: %v", err)
	}

	token := os.Getenv(apiTokenEnvVar)
	if token == "" {
		log.Fatalf("The environment variable $%s must be set. Exiting.\n", apiTokenEnvVar)

	}
	s.FossaClient = fossa.NewClient(token)
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
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "could not parse event", http.StatusBadRequest)
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
				projectName, titleErr := GetProjectNameFromProjectTitle(e.Issue.GetTitle())
				if titleErr != nil {
					log.Printf("failed to parse projecN name: %v", err)
					continue
				}
				// Get Project from db
				var project model.Project
				project = s.Projects[projectName]
				if err := signProjectUpForFOSSA(r.Context(), s.Store, s.FossaClient, project); err != nil {
					log.Printf("failed to send FOSSA invitations: %v", err)
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func signProjectUpForFOSSA(context context.Context, store *db.SQLStore, client *fossa.Client, project model.Project) error {
	log.Printf("Signing project up for fossa")
	return nil
}
