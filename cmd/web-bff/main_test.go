package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("maintainerd_test"),
		postgres.WithUsername("maintainerd"),
		postgres.WithPassword("maintainerd"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	var gormDB *gorm.DB
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		gormDB, lastErr = db.OpenGorm("postgres", dsn, &gorm.Config{
			Logger:                                   logger.Default.LogMode(logger.Silent),
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if lastErr != nil {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			continue
		}
		sqlDB, err := gormDB.DB()
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = sqlDB.PingContext(pingCtx)
			cancel()
		}
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	require.NoError(t, lastErr)

	err = gormDB.AutoMigrate(
		&model.AuditLog{},
		&model.Company{},
		&model.Foundation{},
		&model.Project{},
		&model.Maintainer{},
		&model.StaffMember{},
		&model.FoundationOfficer{},
		&model.Collaborator{},
		&model.MaintainerProject{},
		&model.Service{},
		&model.RemoteTeam{},
		&model.RemoteUser{},
		&model.RemoteTeamUser{},
		&model.ServiceInvitation{},
	)
	require.NoError(t, err)

	return gormDB
}

func performMaintainerGet(t *testing.T, s *server, maintainerID uint, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/maintainers/%d", maintainerID), nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleMaintainer))
	handler.ServeHTTP(rec, req)
	return rec
}

func TestMaintainerEmailRedaction(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	company := model.Company{Name: "Test Co"}
	require.NoError(t, dbConn.Create(&company).Error)

	project := model.Project{Name: "Project A", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)

	alice := model.Maintainer{
		Name:             "Alice Example",
		Email:            "alice@example.org",
		GitHubAccount:    "alice-example",
		GitHubEmail:      "alice@github.example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&alice).Error)

	bob := model.Maintainer{
		Name:             "Bob Example",
		Email:            "bob@example.org",
		GitHubAccount:    "bob-example",
		GitHubEmail:      "bob@github.example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&bob).Error)

	require.NoError(t, dbConn.Model(&project).Association("Maintainers").Append(&alice, &bob))

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	aliceSessionID := "alice-session"
	s.sessions.Set(session{
		ID:        aliceSessionID,
		Login:     alice.GitHubAccount,
		Role:      roleMaintainer,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	t.Run("staff sees all maintainer emails", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, staffSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Equal(t, bob.Email, response.Email)
		assert.Equal(t, bob.GitHubEmail, response.GitHubEmail)
	})

	t.Run("maintainer sees own email", func(t *testing.T) {
		rec := performMaintainerGet(t, s, alice.ID, aliceSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Equal(t, alice.Email, response.Email)
		assert.Equal(t, alice.GitHubEmail, response.GitHubEmail)
	})

	t.Run("maintainer cannot see other maintainer email", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, aliceSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Empty(t, response.Email)
		assert.Empty(t, response.GitHubEmail)
	})
}

func TestMaintainerCanAccessAllProjectsAndMaintainers(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	company := model.Company{Name: "Test Co"}
	require.NoError(t, dbConn.Create(&company).Error)

	projectA := model.Project{Name: "Project A", Maturity: model.Sandbox}
	projectB := model.Project{Name: "Project B", Maturity: model.Graduated}
	require.NoError(t, dbConn.Create(&projectA).Error)
	require.NoError(t, dbConn.Create(&projectB).Error)

	alice := model.Maintainer{
		Name:             "Alice Example",
		Email:            "alice@example.org",
		GitHubAccount:    "alice-example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	bob := model.Maintainer{
		Name:             "Bob Example",
		Email:            "bob@example.org",
		GitHubAccount:    "bob-example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&alice).Error)
	require.NoError(t, dbConn.Create(&bob).Error)
	require.NoError(t, dbConn.Model(&projectA).Association("Maintainers").Append(&alice))

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	maintainerSessionID := "alice-session"
	s.sessions.Set(session{
		ID:        maintainerSessionID,
		Login:     alice.GitHubAccount,
		Role:      roleMaintainer,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	t.Run("maintainer can access any project", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%d", projectB.ID), nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: maintainerSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleProject))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("maintainer can list all projects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: maintainerSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleProjects))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var response projectsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.GreaterOrEqual(t, response.Total, int64(2))
	})

	t.Run("maintainer can access other maintainer profiles", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, maintainerSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestParseGitHubIssueURL(t *testing.T) {
	t.Run("parses github issue url", func(t *testing.T) {
		owner, repo, number, err := parseGitHubIssueURL("https://github.com/cncf/sandbox/issues/1234")
		require.NoError(t, err)
		assert.Equal(t, "cncf", owner)
		assert.Equal(t, "sandbox", repo)
		assert.Equal(t, 1234, number)
	})

	t.Run("rejects non-github host", func(t *testing.T) {
		_, _, _, err := parseGitHubIssueURL("https://example.com/cncf/sandbox/issues/1")
		require.Error(t, err)
	})

	t.Run("rejects non-issue urls", func(t *testing.T) {
		_, _, _, err := parseGitHubIssueURL("https://github.com/cncf/sandbox/pulls/12")
		require.Error(t, err)
	})

	t.Run("rejects missing number", func(t *testing.T) {
		_, _, _, err := parseGitHubIssueURL("https://github.com/cncf/sandbox/issues/")
		require.Error(t, err)
	})
}

func TestHandleProjectCreate(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	s := &server{
		store:       store,
		sessions:    newSessionStore(log.New(io.Discard, "", 0)),
		cookieName:  defaultSessionCookieName,
		logger:      log.New(io.Discard, "", 0),
		githubToken: "test-token",
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	s.fetchIssueTitle = func(_ context.Context, _, _ string, _ int) (string, error) {
		return "[PROJECT ONBOARDING] Example Project", nil
	}

	body := `{"onboardingIssue":"https://github.com/cncf/sandbox/issues/123","legacyMaintainerRef":"https://github.com/exampleorg/example/blob/main/OWNERS"}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleProjects))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response projectCreateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "Example Project", response.Name)
	assert.Equal(t, "Sandbox", response.Maturity)
	assert.Equal(t, "exampleorg", response.GitHubOrg)
}

func TestHandleProjectsNamePrefix(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	projectA := model.Project{Name: "KubeFlow", Maturity: model.Sandbox}
	projectB := model.Project{Name: "Argo", Maturity: model.Graduated}
	projectC := model.Project{Name: "KubeEdge", Maturity: model.Incubating}
	require.NoError(t, dbConn.Create(&projectA).Error)
	require.NoError(t, dbConn.Create(&projectB).Error)
	require.NoError(t, dbConn.Create(&projectC).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/projects?namePrefix=ku", nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleProjects))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response projectsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	names := make([]string, 0, len(response.Projects))
	for _, project := range response.Projects {
		names = append(names, project.Name)
	}
	assert.ElementsMatch(t, []string{"KubeFlow", "KubeEdge"}, names)
}

func TestHandleOnboardingIssues(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	s := &server{
		store:           store,
		sessions:        newSessionStore(log.New(io.Discard, "", 0)),
		cookieName:      defaultSessionCookieName,
		logger:          log.New(io.Discard, "", 0),
		githubToken:     "token",
		onboardingCache: &onboardingIssueCache{},
	}

	s.fetchIssues = func(ctx context.Context) ([]onboardingIssueSummary, error) {
		return []onboardingIssueSummary{
			{
				Number:      101,
				Title:       "[PROJECT ONBOARDING] Sample",
				URL:         "https://github.com/cncf/sandbox/issues/101",
				ProjectName: "Sample",
			},
		}, nil
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/onboarding/issues", nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleOnboardingIssues))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response onboardingIssuesResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	require.Len(t, response.Issues, 1)
	assert.Equal(t, 101, response.Issues[0].Number)
	assert.Equal(t, "Sample", response.Issues[0].ProjectName)
}

func TestHandleMaintainerFromRef_AuditLog(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	project := model.Project{Name: "Cedar", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)

	existing := model.Maintainer{
		Name:             "",
		Email:            "sam.quill@example.invalid",
		GitHubAccount:    "GITHUB_MISSING",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&existing).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	body := `{"projectId":` + fmt.Sprintf("%d", project.ID) + `,"name":"Sam Quill","githubHandle":"samquill","email":"sam.quill@example.invalid","company":"Acme Co"}`
	req := httptest.NewRequest(http.MethodPost, "/api/maintainers/from-ref", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleMaintainerFromRef))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var audit model.AuditLog
	err := dbConn.Where("maintainer_id = ? AND project_id = ? AND action = ?", existing.ID, project.ID, "MAINTAINER_UPDATE").First(&audit).Error
	require.NoError(t, err)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(audit.Metadata), &metadata))
	changes, ok := metadata["changes"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, changes, "name")
	assert.Contains(t, changes, "github")
	assert.Contains(t, changes, "company")
}

func TestFossaInviteEndpointsContract(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now().UTC()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	project := model.Project{Name: "Fossa Project", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)

	service := model.Service{Name: "FOSSA", Description: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)

	serviceTeamName := "CNCF Test"
	serviceTeam := model.RemoteTeam{
		ProjectID:      project.ID,
		ServiceID:      service.ID,
		RemoteTeamID:   101,
		RemoteTeamName: &serviceTeamName,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	invite := model.ServiceInvitation{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		ServiceEmail: "maintainer@example.org",
		RemoteTeamID: 101,
		Status:       "error",
		SentAt:       &now,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	t.Run("list invites returns expected shape", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/services/fossa/invites?projectId=%d", project.ID), nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleFossaInvites))
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var response []fossaInviteSummary
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		require.Len(t, response, 1)
		assert.Equal(t, invite.ID, response[0].ID)
		assert.Equal(t, invite.ServiceEmail, response[0].Email)
		assert.Equal(t, invite.RemoteTeamID, response[0].FossaTeamID)
		assert.Equal(t, serviceTeamName, response[0].FossaTeamName)
		assert.Equal(t, invite.Status, response[0].Status)
	})

	t.Run("invite requires project id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/services/fossa/invite", strings.NewReader(`{}`))
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleFossaInvite))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("reissue without token returns server error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/services/fossa/invites/%d/reissue", invite.ID), nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleFossaInviteAction))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "FOSSA_API_TOKEN not set")
	})
}

func TestFossaChooseRequiresRemoteTeamIDFromFossa(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now().UTC()

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	project := model.Project{Name: "Fossa Project", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)

	service := model.Service{Name: "FOSSA", Description: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
		testMode:   true,
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/services/fossa/choose", strings.NewReader(
		fmt.Sprintf(`{"projectId":%d}`, project.ID),
	))
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: staffSessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleFossaChoose))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "RemoteTeamID must come from FOSSA API")

	var count int64
	require.NoError(t, dbConn.Model(&model.RemoteTeam{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
