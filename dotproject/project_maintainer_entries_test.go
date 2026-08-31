package dotproject

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maintainerEntriesFixture = `maintainers:
  - teams:
      - name: project-maintainers
        members:
          - alice-example
          - bob-example
      - name: other-team
        members:
          - carol-example
`

func TestParseProjectMaintainerEntriesReturnsLineNumbers(t *testing.T) {
	entries, status, parseErr := ParseProjectMaintainerEntries(maintainerEntriesFixture)
	require.Equal(t, ParseStatusParsed, status, parseErr)
	require.Len(t, entries, 2)

	byHandle := make(map[string]int, len(entries))
	for _, entry := range entries {
		byHandle[entry.Handle] = entry.Line
	}
	assert.Equal(t, 5, byHandle["alice-example"])
	assert.Equal(t, 6, byHandle["bob-example"])
	assert.NotContains(t, byHandle, "carol-example")
}

func TestParseProjectMaintainerHandlesMatchesEntriesWrapper(t *testing.T) {
	entries, entryStatus, entryErr := ParseProjectMaintainerEntries(maintainerEntriesFixture)
	require.Equal(t, ParseStatusParsed, entryStatus, entryErr)

	handles, status, parseErr := ParseProjectMaintainerHandles(maintainerEntriesFixture)
	require.Equal(t, ParseStatusParsed, status, parseErr)

	expected := make([]string, 0, len(entries))
	for _, entry := range entries {
		expected = append(expected, entry.Handle)
	}
	sort.Strings(expected)
	sort.Strings(handles)
	assert.Equal(t, expected, handles)
}

func TestParseProjectMaintainerEntriesNoTeamMembers(t *testing.T) {
	_, status, parseErr := ParseProjectMaintainerEntries(`maintainers:
  - teams:
      - name: other-team
        members:
          - carol-example
`)
	assert.Equal(t, ParseStatusInvalidShape, status)
	assert.Equal(t, "no project-maintainers team found", parseErr)
}

func TestParseProjectMaintainerEntriesTeamPresentButEmpty(t *testing.T) {
	_, status, parseErr := ParseProjectMaintainerEntries(`maintainers:
  - teams:
      - name: project-maintainers
        members: []
`)
	assert.Equal(t, ParseStatusInvalidShape, status)
	assert.Equal(t, "project-maintainers team has no members", parseErr)
}

func TestParseProjectMaintainerEntriesNearMissTeamName(t *testing.T) {
	_, status, parseErr := ParseProjectMaintainerEntries(`maintainers:
  - teams:
      - name: Project Maintainers
        members:
          - carol-example
`)
	assert.Equal(t, ParseStatusInvalidShape, status)
	assert.Equal(t, "no project-maintainers team found; saw maintainer-like team name(s) that did not match exactly: Project Maintainers", parseErr)
}
