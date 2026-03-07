package main

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"maintainerd/db"
	"maintainerd/model"
	"maintainerd/plugins/fossa"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeFossaClient struct {
	hasPending bool
	userID     uint
	teamEmails []string
	addResp    *fossa.TeamAddResponse
	addErr     error
	rawMembers fossa.TeamMembers
	rawBody    []byte
}

func (f *fakeFossaClient) HasPendingInvitation(string) (bool, error) { return f.hasPending, nil }
func (f *fakeFossaClient) SendUserInvitation(string) error           { return nil }
func (f *fakeFossaClient) AddUserToTeamByEmail(uint, string, int) error {
	return f.addErr
}
func (f *fakeFossaClient) AddUserToTeamByEmailWithResponse(uint, string, int) (*fossa.TeamAddResponse, error) {
	if f.addResp != nil || f.addErr != nil {
		return f.addResp, f.addErr
	}
	return &fossa.TeamAddResponse{Status: "200 OK", Body: []byte(`{"action":"add"}`), UserID: f.userID, RoleID: 3}, nil
}
func (f *fakeFossaClient) FetchTeamUserEmails(uint) ([]string, error) { return f.teamEmails, nil }
func (f *fakeFossaClient) FetchTeamMembersRaw(uint) (fossa.TeamMembers, []byte, error) {
	if f.rawBody != nil || f.rawMembers.TotalCount > 0 || len(f.rawMembers.Results) > 0 {
		return f.rawMembers, f.rawBody, nil
	}
	body := []byte(`{"results":[{"email":"dana@example.com"}],"totalCount":1}`)
	return fossa.TeamMembers{Results: []fossa.TeamMember{{Email: "dana@example.com"}}, TotalCount: 1}, body, nil
}
func (f *fakeFossaClient) FindUserIDByEmail(string) (uint, error) { return f.userID, nil }

func setupPollerTestDB(t *testing.T) *gorm.DB {
	dbConn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, dbConn.AutoMigrate(
		&model.Service{},
		&model.Project{},
		&model.Maintainer{},
		&model.Foundation{},
		&model.StaffMember{},
		&model.AuditLog{},
		&model.RemoteTeam{},
		&model.ServiceInvitation{},
		&model.RemoteUser{},
		&model.RemoteTeamUser{},
	))
	return dbConn
}

func TestPollerRecordsRemoteTeamMembership(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "composefs", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Dana Dev",
		Email:            "dana@example.com",
		GitHubAccount:    "danadev",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 123456,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-1 * time.Hour)
	invite := model.ServiceInvitation{
		ServiceID:    service.ID,
		ProjectID:    project.ID,
		MaintainerID: &maintainer.ID,
		RemoteTeamID: serviceTeam.RemoteTeamID,
		ServiceEmail: maintainer.Email,
		Status:       "pending",
		SentAt:       &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	client := &fakeFossaClient{hasPending: false, userID: 777, teamEmails: []string{maintainer.Email}}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "accepted", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "done", *updated.TeamAssignmentStatus)
	require.Equal(t, 0, updated.TeamAddAttempts)

	var user model.RemoteUser
	require.NoError(t, dbConn.Where("service_id = ? AND remote_user_id = ?", service.ID, 777).First(&user).Error)
	require.Equal(t, maintainer.Email, user.ServiceEmail)

	var link model.RemoteTeamUser
	require.NoError(t, dbConn.Where(
		"service_id = ? AND team_id = ? AND user_id = ?",
		service.ID, serviceTeam.ID, user.ID,
	).First(&link).Error)
	require.NotNil(t, link.MaintainerID)
	require.Equal(t, maintainer.ID, *link.MaintainerID)
}

func TestPollerTeamAssignmentPending(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "flux", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Matheus Pimenta",
		Email:            "matheus@example.com",
		GitHubAccount:    "matheuscscp",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 646,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	invite := model.ServiceInvitation{
		ServiceID:    service.ID,
		ProjectID:    project.ID,
		MaintainerID: &maintainer.ID,
		RemoteTeamID: serviceTeam.RemoteTeamID,
		ServiceEmail: maintainer.Email,
		Status:       "pending",
		SentAt:       &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	client := &fakeFossaClient{
		hasPending: false,
		userID:     777,
		teamEmails: []string{},
		rawMembers: fossa.TeamMembers{Results: []fossa.TeamMember{}, TotalCount: 0},
		rawBody:    []byte(`{"results":[],"totalCount":0}`),
	}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "accepted", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "pending", *updated.TeamAssignmentStatus)
	require.Equal(t, 1, updated.TeamAddAttempts)
	require.NotNil(t, updated.NextTeamAddAt)
}

func TestPollerTeamAssignmentFailureAfterRetries(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "flux", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Matheus Pimenta",
		Email:            "matheus@example.com",
		GitHubAccount:    "matheuscscp",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 646,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	pendingStatus := "pending"
	nextAt := time.Now().UTC().Add(-1 * time.Minute)
	invite := model.ServiceInvitation{
		ServiceID:            service.ID,
		ProjectID:            project.ID,
		MaintainerID:         &maintainer.ID,
		RemoteTeamID:         serviceTeam.RemoteTeamID,
		ServiceEmail:         maintainer.Email,
		Status:               "accepted",
		TeamAssignmentStatus: &pendingStatus,
		TeamAddAttempts:      2,
		NextTeamAddAt:        &nextAt,
		SentAt:               &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	client := &fakeFossaClient{
		hasPending: false,
		userID:     777,
		teamEmails: []string{},
		rawMembers: fossa.TeamMembers{Results: []fossa.TeamMember{}, TotalCount: 0},
		rawBody:    []byte(`{"results":[],"totalCount":0}`),
	}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "accepted", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "error", *updated.TeamAssignmentStatus)
	require.Equal(t, 3, updated.TeamAddAttempts)
	require.Nil(t, updated.NextTeamAddAt)
	require.NotNil(t, updated.LastError)

	var audit model.AuditLog
	require.NoError(t, dbConn.Where("action = ?", "FOSSA_TEAM_MEMBER_ADD_FAILED").First(&audit).Error)
}

func TestPollerTeamAssignmentSuccessAfterRetry(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "flux", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Matheus Pimenta",
		Email:            "matheus@example.com",
		GitHubAccount:    "matheuscscp",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 646,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	pendingStatus := "pending"
	nextAt := time.Now().UTC().Add(-1 * time.Minute)
	invite := model.ServiceInvitation{
		ServiceID:            service.ID,
		ProjectID:            project.ID,
		MaintainerID:         &maintainer.ID,
		RemoteTeamID:         serviceTeam.RemoteTeamID,
		ServiceEmail:         maintainer.Email,
		Status:               "accepted",
		TeamAssignmentStatus: &pendingStatus,
		TeamAddAttempts:      1,
		NextTeamAddAt:        &nextAt,
		SentAt:               &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	client := &fakeFossaClient{
		hasPending: false,
		userID:     777,
		teamEmails: []string{maintainer.Email},
		rawMembers: fossa.TeamMembers{Results: []fossa.TeamMember{{Email: maintainer.Email}}, TotalCount: 1},
		rawBody:    []byte(`{"results":[{"email":"matheus@example.com"}],"totalCount":1}`),
	}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "accepted", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "done", *updated.TeamAssignmentStatus)
	require.Equal(t, 0, updated.TeamAddAttempts)
	require.Nil(t, updated.NextTeamAddAt)

	var audit model.AuditLog
	require.NoError(t, dbConn.Where("action = ?", "FOSSA_TEAM_MEMBER_ADDED").First(&audit).Error)
}

func TestPollerTeamAssignmentAlreadyMember(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "flux", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Matheus Pimenta",
		Email:            "matheus@example.com",
		GitHubAccount:    "matheuscscp",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 646,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	invite := model.ServiceInvitation{
		ServiceID:    service.ID,
		ProjectID:    project.ID,
		MaintainerID: &maintainer.ID,
		RemoteTeamID: serviceTeam.RemoteTeamID,
		ServiceEmail: maintainer.Email,
		Status:       "pending",
		SentAt:       &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	addResp := &fossa.TeamAddResponse{Status: "200 OK", Body: []byte(`{"code":2001}`), UserID: 777, RoleID: 3}
	client := &fakeFossaClient{
		hasPending: false,
		userID:     777,
		addResp:    addResp,
		addErr:     fossa.ErrUserAlreadyMember,
		teamEmails: []string{maintainer.Email},
		rawMembers: fossa.TeamMembers{Results: []fossa.TeamMember{{Email: maintainer.Email}}, TotalCount: 1},
		rawBody:    []byte(`{"results":[{"email":"matheus@example.com"}],"totalCount":1}`),
	}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "accepted", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "done", *updated.TeamAssignmentStatus)

	var audit model.AuditLog
	require.NoError(t, dbConn.Where("action = ?", "FOSSA_TEAM_MEMBER_ADDED").First(&audit).Error)
}

func TestPollerTeamAssignmentImmediateAddError(t *testing.T) {
	dbConn := setupPollerTestDB(t)
	store := db.NewSQLStore(dbConn)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, dbConn.Create(&service).Error)
	project := model.Project{Name: "flux", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Matheus Pimenta",
		Email:            "matheus@example.com",
		GitHubAccount:    "matheuscscp",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, dbConn.Create(&maintainer).Error)

	serviceTeam := model.RemoteTeam{
		ProjectID:    project.ID,
		ServiceID:    service.ID,
		RemoteTeamID: 646,
	}
	require.NoError(t, dbConn.Create(&serviceTeam).Error)

	sentAt := time.Now().UTC().Add(-10 * time.Minute)
	invite := model.ServiceInvitation{
		ServiceID:    service.ID,
		ProjectID:    project.ID,
		MaintainerID: &maintainer.ID,
		RemoteTeamID: serviceTeam.RemoteTeamID,
		ServiceEmail: maintainer.Email,
		Status:       "pending",
		SentAt:       &sentAt,
	}
	require.NoError(t, dbConn.Create(&invite).Error)

	addResp := &fossa.TeamAddResponse{Status: "400 Bad Request", Body: []byte(`{"error":"boom"}`), UserID: 777, RoleID: 3}
	client := &fakeFossaClient{
		hasPending: false,
		userID:     777,
		addResp:    addResp,
		addErr:     errors.New("boom"),
		teamEmails: []string{},
		rawMembers: fossa.TeamMembers{Results: []fossa.TeamMember{}, TotalCount: 0},
		rawBody:    []byte(`{"results":[],"totalCount":0}`),
	}
	logger := log.New(io.Discard, "", 0)

	err := pollFossaInvites(context.Background(), logger, store, client)
	require.NoError(t, err)

	var updated model.ServiceInvitation
	require.NoError(t, dbConn.First(&updated, invite.ID).Error)
	require.Equal(t, "error", updated.Status)
	require.NotNil(t, updated.TeamAssignmentStatus)
	require.Equal(t, "error", *updated.TeamAssignmentStatus)

	var audit model.AuditLog
	require.NoError(t, dbConn.Where("action = ?", "FOSSA_TEAM_MEMBER_ADD_FAILED").First(&audit).Error)
}
