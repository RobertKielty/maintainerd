package db

import (
	"maintainerd/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.Company{},
		&model.Project{},
		&model.Maintainer{},
		&model.MaintainerProject{},
		&model.Service{},
		&model.RemoteTeam{},
		&model.RemoteUser{},
		&model.RemoteTeamUser{},
	)
	require.NoError(t, err)

	return db
}

// seedTestData creates test fixtures in the database
func seedTestData(t *testing.T, db *gorm.DB) (company model.Company, project1, project2 model.Project, maintainer1, maintainer2, maintainer3 model.Maintainer) {
	// Create a company
	company = model.Company{Name: "Test Company"}
	require.NoError(t, db.Create(&company).Error)

	// Create projects
	project1 = model.Project{Name: "kubernetes", Maturity: model.Graduated}
	require.NoError(t, db.Create(&project1).Error)

	project2 = model.Project{Name: "prometheus", Maturity: model.Graduated}
	require.NoError(t, db.Create(&project2).Error)

	// Create maintainers
	maintainer1 = model.Maintainer{
		Name:             "Alice Developer",
		Email:            "alice@example.com",
		GitHubAccount:    "alice",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, db.Create(&maintainer1).Error)

	maintainer2 = model.Maintainer{
		Name:             "Bob Engineer",
		Email:            "bob@example.com",
		GitHubAccount:    "bob",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, db.Create(&maintainer2).Error)

	maintainer3 = model.Maintainer{
		Name:             "Charlie Contributor",
		Email:            "charlie@example.com",
		GitHubAccount:    "charlie",
		MaintainerStatus: model.EmeritusMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, db.Create(&maintainer3).Error)

	// Associate maintainers with projects
	// project1 has maintainer1 and maintainer2
	require.NoError(t, db.Model(&project1).Association("Maintainers").Append(&maintainer1, &maintainer2))

	// project2 has maintainer2 and maintainer3
	require.NoError(t, db.Model(&project2).Association("Maintainers").Append(&maintainer2, &maintainer3))

	return
}

func TestGetMaintainersByProject(t *testing.T) {
	db := setupTestDB(t)
	company, project1, project2, maintainer1, maintainer2, maintainer3 := seedTestData(t, db)
	store := NewSQLStore(db)

	t.Run("returns maintainers for project with multiple maintainers", func(t *testing.T) {
		maintainers, err := store.GetMaintainersByProject(project1.ID)
		require.NoError(t, err)
		require.Len(t, maintainers, 2)

		// Verify maintainer data
		maintainerIDs := []uint{maintainers[0].ID, maintainers[1].ID}
		assert.Contains(t, maintainerIDs, maintainer1.ID)
		assert.Contains(t, maintainerIDs, maintainer2.ID)

		// Verify Company is preloaded
		for _, m := range maintainers {
			assert.NotNil(t, m.CompanyID)
			assert.Equal(t, company.ID, m.Company.ID)
			assert.Equal(t, "Test Company", m.Company.Name)
		}
	})

	t.Run("returns different maintainers for different project", func(t *testing.T) {
		maintainers, err := store.GetMaintainersByProject(project2.ID)
		require.NoError(t, err)
		require.Len(t, maintainers, 2)

		maintainerIDs := []uint{maintainers[0].ID, maintainers[1].ID}
		assert.Contains(t, maintainerIDs, maintainer2.ID)
		assert.Contains(t, maintainerIDs, maintainer3.ID)
	})

	t.Run("returns empty slice for project with no maintainers", func(t *testing.T) {
		emptyProject := model.Project{Name: "empty-project", Maturity: model.Sandbox}
		require.NoError(t, db.Create(&emptyProject).Error)

		maintainers, err := store.GetMaintainersByProject(emptyProject.ID)
		require.NoError(t, err)
		assert.Empty(t, maintainers)
	})

	t.Run("returns empty slice for non-existent project", func(t *testing.T) {
		maintainers, err := store.GetMaintainersByProject(99999)
		require.Error(t, err)
		assert.Equal(t, ErrProjectNotFound, err)
		assert.Nil(t, maintainers)
	})

	t.Run("maintainers have correct fields populated", func(t *testing.T) {
		maintainers, err := store.GetMaintainersByProject(project1.ID)
		require.NoError(t, err)
		require.NotEmpty(t, maintainers)

		m := maintainers[0]
		assert.NotEmpty(t, m.Name)
		assert.NotEmpty(t, m.Email)
		assert.NotEmpty(t, m.GitHubAccount)
		assert.True(t, m.MaintainerStatus.IsValid())
		assert.NotNil(t, m.CompanyID)

		// Projects field should NOT be populated (not preloaded)
		assert.Empty(t, m.Projects)
	})
}

func TestGetProjectsUsingService(t *testing.T) {
	t.Skip("testDB not defined - needs implementation")
}

func TestUpsertMaintainer_FillsMissingFields(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLStore(db)

	project := model.Project{Name: "cedar", Maturity: model.Sandbox}
	require.NoError(t, db.Create(&project).Error)

	// Existing maintainer with missing name and GitHub account.
	existing := model.Maintainer{
		Name:             "",
		Email:            "accorell@amazon.com",
		GitHubAccount:    "GITHUB_MISSING",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, db.Create(&existing).Error)

	maintainer, err := store.UpsertMaintainer(project.ID, "Adrian Palacios", "accorell@amazon.com", "adpaco-aws", "Amazon")
	require.NoError(t, err)
	require.NotNil(t, maintainer)
	assert.Equal(t, existing.ID, maintainer.ID)

	var refreshed model.Maintainer
	require.NoError(t, db.First(&refreshed, existing.ID).Error)
	assert.Equal(t, "Adrian Palacios", refreshed.Name)
	assert.Equal(t, "adpaco-aws", refreshed.GitHubAccount)
	assert.Equal(t, "accorell@amazon.com", refreshed.Email)
	assert.NotNil(t, refreshed.CompanyID)
	assert.NotEqual(t, uint(0), *refreshed.CompanyID)

	var count int64
	require.NoError(t, db.Model(&model.MaintainerProject{}).
		Where("maintainer_id = ? AND project_id = ?", existing.ID, project.ID).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestUpsertRemoteUser(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLStore(db)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, db.Create(&service).Error)

	user := &model.RemoteUser{
		ServiceID:    service.ID,
		RemoteUserID: 101,
		ServiceEmail: "alice@example.com",
		RemoteRef:    "alice-ref",
	}
	created, err := store.UpsertRemoteUser(user)
	require.NoError(t, err)
	require.NotNil(t, created)

	userUpdate := &model.RemoteUser{
		ServiceID:    service.ID,
		RemoteUserID: 101,
		ServiceEmail: "alice+updated@example.com",
		RemoteRef:    "alice-ref-updated",
	}
	updated, err := store.UpsertRemoteUser(userUpdate)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "alice+updated@example.com", updated.ServiceEmail)
	assert.Equal(t, "alice-ref-updated", updated.RemoteRef)

	var count int64
	require.NoError(t, db.Model(&model.RemoteUser{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestUpsertRemoteUserTeam(t *testing.T) {
	db := setupTestDB(t)
	store := NewSQLStore(db)

	service := model.Service{Name: "FOSSA"}
	require.NoError(t, db.Create(&service).Error)
	project := model.Project{Name: "kubernetes", Maturity: model.Sandbox}
	require.NoError(t, db.Create(&project).Error)
	maintainer := model.Maintainer{
		Name:             "Alice Developer",
		Email:            "alice@example.com",
		GitHubAccount:    "alice",
		MaintainerStatus: model.ActiveMaintainer,
	}
	require.NoError(t, db.Create(&maintainer).Error)

	remoteTeam := model.RemoteTeam{
		ServiceID:      service.ID,
		ProjectID:      project.ID,
		RemoteTeamID:   999001,
		RemoteTeamName: strPtr("Team Demo"),
	}
	require.NoError(t, db.Create(&remoteTeam).Error)

	remoteUser := model.RemoteUser{
		ServiceID:    service.ID,
		RemoteUserID: 101,
		ServiceEmail: "alice@example.com",
		RemoteRef:    "alice-ref",
	}
	require.NoError(t, db.Create(&remoteUser).Error)

	link := &model.RemoteTeamUser{
		ServiceID:    service.ID,
		TeamID:       remoteTeam.ID,
		UserID:       remoteUser.ID,
		MaintainerID: &maintainer.ID,
	}
	created, err := store.UpsertRemoteUserTeam(link)
	require.NoError(t, err)
	require.NotNil(t, created)

	updatedLink := &model.RemoteTeamUser{
		ServiceID:    service.ID,
		TeamID:       remoteTeam.ID,
		UserID:       remoteUser.ID,
		MaintainerID: &maintainer.ID,
	}
	updated, err := store.UpsertRemoteUserTeam(updatedLink)
	require.NoError(t, err)
	require.NotNil(t, updated)

	var count int64
	require.NoError(t, db.Model(&model.RemoteTeamUser{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func strPtr(value string) *string {
	return &value
}
