package dotproject

import (
	"context"
	"fmt"
	"testing"

	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepositoryClient struct {
	defaultBranches map[string]string
	files           map[string]*FetchedFile
}

func (f *fakeRepositoryClient) GetDefaultBranch(_ context.Context, owner, repo string) (string, error) {
	key := owner + "/" + repo
	branch, ok := f.defaultBranches[key]
	if !ok {
		return "", ErrDotProjectRepoNotFound
	}
	return branch, nil
}

func (f *fakeRepositoryClient) GetFile(_ context.Context, owner, repo, ref, path string) (*FetchedFile, error) {
	key := fmt.Sprintf("%s/%s@%s:%s", owner, repo, ref, path)
	file, ok := f.files[key]
	if !ok {
		return nil, ErrDotProjectFileNotFound
	}
	return file, nil
}

func TestDiscoverRepoMissing(t *testing.T) {
	t.Parallel()

	discoverer := &Discoverer{
		Client: &fakeRepositoryClient{
			defaultBranches: map[string]string{},
			files:           map[string]*FetchedFile{},
		},
	}

	result, err := discoverer.Discover(context.Background(), model.Project{GitHubOrg: "example-org"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.RepoExists)
	assert.Equal(t, "https://github.com/example-org/.project", result.RepoRef)
	assert.Equal(t, ParseStatusMissing, result.ProjectParseStatus)
	assert.Equal(t, ParseStatusMissing, result.MaintainersParseStatus)
}

func TestDiscoverLowercaseMaintainersFileAndDeduplicatesHandles(t *testing.T) {
	t.Parallel()

	client := &fakeRepositoryClient{
		defaultBranches: map[string]string{
			"example-org/.project": "main",
		},
		files: map[string]*FetchedFile{
			"example-org/.project@main:project.yaml": {
				Path: "project.yaml",
				Body: "schema_version: \"1.0.0\"\nname: Example\n",
			},
			"example-org/.project@main:maintainers.yaml": {
				Path: "maintainers.yaml",
				Body: `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - Alice
          - "@bob"
      - name: sig-release
        members:
          - alice
          - CAROL
`,
			},
			"example-org/.project@main:SECURITY.md": {
				Path: "SECURITY.md",
				Body: "# Security\n",
			},
		},
	}

	discoverer := &Discoverer{Client: client}
	result, err := discoverer.Discover(context.Background(), model.Project{GitHubOrg: "example-org"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.RepoExists)
	assert.Equal(t, "main", result.DefaultBranch)
	assert.True(t, result.ProjectFile.Exists)
	assert.True(t, result.MaintainersFile.Exists)
	assert.Equal(t, "maintainers.yaml", result.MaintainersFilename)
	assert.Equal(t, SupportedSchemaVersion, result.SchemaVersion)
	assert.True(t, result.SchemaSupported)
	assert.Equal(t, ParseStatusParsed, result.ProjectParseStatus)
	assert.Equal(t, ParseStatusParsed, result.MaintainersParseStatus)
	require.NotNil(t, result.MaintainerCount)
	assert.Equal(t, uint(3), *result.MaintainerCount)
	assert.True(t, result.SecurityFile.Exists)
	assert.False(t, result.ContributingFile.Exists)
	assert.False(t, result.GovernanceFile.Exists)
}

func TestDiscoverUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	client := &fakeRepositoryClient{
		defaultBranches: map[string]string{
			"example-org/.project": "main",
		},
		files: map[string]*FetchedFile{
			"example-org/.project@main:project.yaml": {
				Path: "project.yaml",
				Body: "schema_version: \"2.0.0\"\nname: Example\n",
			},
			"example-org/.project@main:MAINTAINERS.yaml": {
				Path: "MAINTAINERS.yaml",
				Body: `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - maintainer1
`,
			},
		},
	}

	discoverer := &Discoverer{Client: client}
	result, err := discoverer.Discover(context.Background(), model.Project{GitHubOrg: "example-org"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "2.0.0", result.SchemaVersion)
	assert.False(t, result.SchemaSupported)
	assert.Equal(t, ParseStatusUnsupportedSchema, result.ProjectParseStatus)
	assert.Contains(t, result.ProjectParseError, "unsupported schema version")
	assert.Equal(t, "MAINTAINERS.yaml", result.MaintainersFilename)
	require.NotNil(t, result.MaintainerCount)
	assert.Equal(t, uint(1), *result.MaintainerCount)
}
