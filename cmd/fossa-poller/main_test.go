package main

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeFossaClient struct {
	hasPending bool
	userID     uint
	teamEmails []string
}

func (f *fakeFossaClient) HasPendingInvitation(string) (bool, error) { return f.hasPending, nil }
func (f *fakeFossaClient) SendUserInvitation(string) error           { return nil }
func (f *fakeFossaClient) AddUserToTeamByEmail(uint, string, int) error {
	return nil
}
func (f *fakeFossaClient) FetchTeamUserEmails(uint) ([]string, error) { return f.teamEmails, nil }
func (f *fakeFossaClient) FindUserIDByEmail(string) (uint, error)     { return f.userID, nil }

func setupPollerTestDB(t *testing.T) *gorm.DB {
	dbConn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, dbConn.AutoMigrate(
		&model.Service{},
		&model.Project{},
		&model.Maintainer{},
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
