package dotproject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMaintainerRosterPatchAddsMissingMaintainersAndRemovesPlaceholders(t *testing.T) {
	source := `maintainers:
  - teams:
      - name: project-maintainers
        members:
          # TODO: Add maintainer GitHub handles
          - github-handle
          - alice
`

	patch, err := BuildMaintainerRosterPatch(source, []string{"alice", "Bob", "@carol"})
	require.NoError(t, err)

	assert.Equal(t, []string{"bob", "carol"}, patch.AddedHandles)
	assert.Equal(t, []string{"# TODO: Add maintainer GitHub handles", "- github-handle"}, patch.RemovedPlaceholders)
	assert.Equal(t, `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - alice
          - bob
          - carol
`, patch.Proposed)
}

func TestBuildMaintainerRosterPatchAddsMembersBlockWhenMissing(t *testing.T) {
	source := `maintainers:
  - teams:
      - name: project-maintainers
`

	patch, err := BuildMaintainerRosterPatch(source, []string{"alice"})
	require.NoError(t, err)

	assert.Equal(t, []string{"alice"}, patch.AddedHandles)
	assert.Equal(t, `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - alice
`, patch.Proposed)
}

func TestBuildMaintainerRosterPatchErrorsWhenProjectMaintainersTeamMissing(t *testing.T) {
	_, err := BuildMaintainerRosterPatch("maintainers: []\n", []string{"alice"})
	require.Error(t, err)
}
