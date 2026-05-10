package dotproject

import (
	"context"
	"errors"
	"testing"
	"time"

	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeSyncStore struct {
	projects   []model.Project
	persisted  map[uint]persistedSync
	persistErr error
}

type persistedSync struct {
	patch model.Project
	state *model.DotProjectSyncState
}

func (f *fakeSyncStore) ListProjects() ([]model.Project, error) {
	return append([]model.Project{}, f.projects...), nil
}

func (f *fakeSyncStore) PersistDotProjectSync(projectID uint, patch model.Project, state *model.DotProjectSyncState) error {
	if f.persistErr != nil {
		return f.persistErr
	}
	if f.persisted == nil {
		f.persisted = make(map[uint]persistedSync)
	}
	f.persisted[projectID] = persistedSync{patch: patch, state: state}
	return nil
}

type fakeDiscoveryRunner struct {
	results map[uint]*DiscoveryResult
	errors  map[uint]error
	seen    map[uint]model.Project
}

func (f *fakeDiscoveryRunner) Discover(_ context.Context, project model.Project) (*DiscoveryResult, error) {
	if f.seen == nil {
		f.seen = make(map[uint]model.Project)
	}
	f.seen[project.ID] = project
	if err := f.errors[project.ID]; err != nil {
		return nil, err
	}
	if result, ok := f.results[project.ID]; ok {
		return result, nil
	}
	return &DiscoveryResult{}, nil
}

func TestSyncProjectPersistsAdoptedDiscovery(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{}
	syncer := &Syncer{
		Store: store,
		Discoverer: &fakeDiscoveryRunner{
			results: map[uint]*DiscoveryResult{
				42: {
					RepoExists:             true,
					RepoRef:                "https://github.com/example-org/.project",
					DefaultBranch:          "main",
					ProjectFile:            FileDiscovery{Exists: true, BlobURL: "https://github.com/example-org/.project/blob/main/project.yaml", ETag: "\"project\"", BodyHash: "hash-project"},
					MaintainersFile:        FileDiscovery{Exists: true, Path: "MAINTAINERS.yaml", BlobURL: "https://github.com/example-org/.project/blob/main/MAINTAINERS.yaml", ETag: "\"maintainers\"", BodyHash: "hash-maintainers"},
					SecurityFile:           FileDiscovery{Exists: true, BlobURL: "https://github.com/example-org/.project/blob/main/SECURITY.md", ETag: "\"security\"", BodyHash: "hash-security"},
					ContributingFile:       FileDiscovery{Exists: true, BlobURL: "https://github.com/example-org/.project/blob/main/CONTRIBUTING.md", ETag: "\"contributing\"", BodyHash: "hash-contributing"},
					GovernanceFile:         FileDiscovery{Exists: true, BlobURL: "https://github.com/example-org/.project/blob/main/GOVERNANCE.md", ETag: "\"governance\"", BodyHash: "hash-governance"},
					MaintainersFilename:    "MAINTAINERS.yaml",
					SchemaVersion:          "1.0.0",
					SchemaSupported:        true,
					MaintainerCount:        uintPtr(5),
					ProjectParseStatus:     ParseStatusParsed,
					MaintainersParseStatus: ParseStatusParsed,
				},
			},
		},
		Now: func() time.Time { return now },
	}

	status, err := syncer.SyncProject(context.Background(), model.Project{Model: gorm.Model{ID: 42}, GitHubOrg: "example-org"})
	require.NoError(t, err)
	assert.Equal(t, AdoptionStatusAdopted, status)

	persisted := store.persisted[42]
	assert.Equal(t, "https://github.com/example-org/.project", persisted.patch.DotProjectRepoRef)
	assert.Equal(t, "https://github.com/example-org/.project/blob/main/project.yaml", persisted.patch.DotProjectProjectRef)
	assert.Equal(t, "https://github.com/example-org/.project/blob/main/MAINTAINERS.yaml", persisted.patch.DotProjectMaintainerRef)
	assert.Equal(t, "1.0.0", persisted.patch.DotProjectSchemaVersion)
	assert.Equal(t, AdoptionStatusAdopted, persisted.patch.DotProjectAdoptionStatus)
	require.NotNil(t, persisted.patch.DotProjectLastSyncedAt)
	assert.True(t, persisted.patch.DotProjectLastSyncedAt.Equal(now))
	require.NotNil(t, persisted.patch.DotProjectMaintainerCount)
	assert.Equal(t, uint(5), *persisted.patch.DotProjectMaintainerCount)

	require.NotNil(t, persisted.state)
	assert.True(t, persisted.state.RepoExists)
	assert.True(t, persisted.state.ProjectFileExists)
	assert.True(t, persisted.state.MaintainersFileExists)
	assert.Equal(t, "main", persisted.state.DefaultBranch)
	assert.Equal(t, "MAINTAINERS.yaml", persisted.state.MaintainersFilename)
	assert.Equal(t, "1.0.0", persisted.state.SchemaVersion)
	assert.Equal(t, ImporterVersion, persisted.state.ImporterVersion)
	require.NotNil(t, persisted.state.LastCheckedAt)
	assert.True(t, persisted.state.LastCheckedAt.Equal(now))
	assert.Nil(t, persisted.state.ParseError)
}

func TestSyncProjectPersistsSyncError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{}
	syncer := &Syncer{
		Store: store,
		Discoverer: &fakeDiscoveryRunner{
			errors: map[uint]error{
				9: errors.New("github api timeout"),
			},
		},
		Now: func() time.Time { return now },
	}

	status, err := syncer.SyncProject(context.Background(), model.Project{Model: gorm.Model{ID: 9}, GitHubOrg: "example-org"})
	require.Error(t, err)
	assert.Equal(t, AdoptionStatusError, status)

	persisted := store.persisted[9]
	assert.Equal(t, AdoptionStatusError, persisted.patch.DotProjectAdoptionStatus)
	require.NotNil(t, persisted.patch.DotProjectLastSyncedAt)
	assert.True(t, persisted.patch.DotProjectLastSyncedAt.Equal(now))
	require.NotNil(t, persisted.state)
	require.NotNil(t, persisted.state.SyncError)
	assert.Equal(t, "github api timeout", *persisted.state.SyncError)
}

func TestSyncProjectInfersGitHubOrgFromLegacyMaintainerRef(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{}
	discoverer := &fakeDiscoveryRunner{
		results: map[uint]*DiscoveryResult{
			77: {RepoExists: false},
		},
	}
	syncer := &Syncer{
		Store:      store,
		Discoverer: discoverer,
		Now:        func() time.Time { return now },
	}

	status, err := syncer.SyncProject(context.Background(), model.Project{
		Model:               gorm.Model{ID: 77},
		LegacyMaintainerRef: "https://github.com/example-org/community/blob/main/maintainers.md",
	})
	require.NoError(t, err)
	assert.Equal(t, AdoptionStatusNotFound, status)
	require.Contains(t, discoverer.seen, uint(77))
	assert.Equal(t, "example-org", discoverer.seen[77].GitHubOrg)
}

func TestSyncProjectKeepsOrgEmptyWhenLegacyRefIsNotGitHub(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{}
	discoverer := &fakeDiscoveryRunner{
		errors: map[uint]error{
			88: errors.New("project github org is required"),
		},
	}
	syncer := &Syncer{
		Store:      store,
		Discoverer: discoverer,
		Now:        func() time.Time { return now },
	}

	status, err := syncer.SyncProject(context.Background(), model.Project{
		Model:               gorm.Model{ID: 88},
		LegacyMaintainerRef: "https://gitlab.example.com/group/project/-/blob/main/maintainers.yaml",
	})
	require.Error(t, err)
	assert.Equal(t, AdoptionStatusError, status)
	require.Contains(t, discoverer.seen, uint(88))
	assert.Equal(t, "", discoverer.seen[88].GitHubOrg)
}

func TestSyncAllSummarizesStatuses(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{
		projects: []model.Project{
			{Model: gorm.Model{ID: 1}, GitHubOrg: "org-one"},
			{Model: gorm.Model{ID: 2}, GitHubOrg: "org-two"},
			{Model: gorm.Model{ID: 3}, GitHubOrg: "org-three"},
			{Model: gorm.Model{ID: 4}, GitHubOrg: "org-four"},
		},
	}
	syncer := &Syncer{
		Store: store,
		Discoverer: &fakeDiscoveryRunner{
			results: map[uint]*DiscoveryResult{
				1: {RepoExists: false},
				2: {RepoExists: true},
				3: {RepoExists: true, ProjectFile: FileDiscovery{Exists: true}, MaintainersFile: FileDiscovery{Exists: true}, SchemaSupported: true, MaintainerCount: uintPtr(2)},
			},
			errors: map[uint]error{
				4: errors.New("boom"),
			},
		},
		Now: func() time.Time { return now },
	}

	summary, err := syncer.SyncAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, summary.Loaded)
	assert.Equal(t, 4, summary.Total)
	assert.Equal(t, 3, summary.Synced)
	assert.Equal(t, 1, summary.Errored)
	assert.Equal(t, 1, summary.NotFound)
	assert.Equal(t, 1, summary.RepoOnly)
	assert.Equal(t, 1, summary.Adopted)
	assert.Equal(t, 0, summary.Partial)
	assert.Len(t, summary.ErrorSummaries, 1)
	assert.Contains(t, summary.ErrorSummaries[0], "boom")
}

func TestSyncAllSkipsArchivedAndMaintainerD(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 10, 11, 12, 0, time.UTC)
	store := &fakeSyncStore{
		projects: []model.Project{
			{Model: gorm.Model{ID: 1}, Name: "Project One", Maturity: model.Sandbox, GitHubOrg: "org-one"},
			{Model: gorm.Model{ID: 2}, Name: "maintainer-d", Maturity: model.Graduated, GitHubOrg: "maintainer-d"},
			{Model: gorm.Model{ID: 3}, Name: "Project Archived", Maturity: model.Archived, GitHubOrg: "org-archived"},
		},
	}
	syncer := &Syncer{
		Store: store,
		Discoverer: &fakeDiscoveryRunner{
			results: map[uint]*DiscoveryResult{
				1: {RepoExists: false},
			},
		},
		Now: func() time.Time { return now },
	}

	summary, err := syncer.SyncAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, summary.Loaded)
	assert.Equal(t, 1, summary.Total)
	assert.Equal(t, 2, summary.Skipped)
	assert.Equal(t, 1, summary.SkippedArchived)
	assert.Equal(t, 1, summary.SkippedExcluded)
	assert.Equal(t, 1, summary.Synced)
	assert.Equal(t, 1, summary.NotFound)
	assert.NotContains(t, store.persisted, uint(2))
	assert.NotContains(t, store.persisted, uint(3))
}

func uintPtr(v uint) *uint {
	return &v
}
