package dotproject

import (
	"testing"

	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGistReportMarkdown(t *testing.T) {
	t.Parallel()

	count := uint(3)
	row, ok := BuildGistReportRow(model.Project{Name: "Example"}, &DiscoveryResult{
		RepoExists:             true,
		ProjectFile:            FileDiscovery{Exists: true, BlobURL: "https://github.com/example/.project/blob/main/project.yaml"},
		MaintainersFile:        FileDiscovery{Exists: true, BlobURL: "https://github.com/example/.project/blob/main/maintainers.yaml"},
		MaintainerCount:        &count,
		SecurityFile:           FileDiscovery{Exists: true, BlobURL: "https://github.com/example/.project/blob/main/SECURITY.md"},
		ContributingFile:       FileDiscovery{Exists: false, BlobURL: "https://github.com/example/.project/blob/main/CONTRIBUTING.md"},
		GovernanceFile:         FileDiscovery{Exists: true, RawURL: "https://raw.githubusercontent.com/example/.project/main/GOVERNANCE.md"},
		MaintainersParseStatus: ParseStatusParsed,
	})
	require.True(t, ok)

	markdown, err := GistReportMarkdown([]GistReportRow{row})
	require.NoError(t, err)

	assert.Equal(t, "| Project Name | project.yaml | maintainers.yaml | Maintainer Count | SECURITY.md | CONTRIBUTING.md | GOVERNANCE.md | Warning |\n| --- | --- | --- | ---: | --- | --- | --- | --- |\n| Example | [project.yaml](https://github.com/example/.project/blob/main/project.yaml) | [maintainers.yaml](https://github.com/example/.project/blob/main/maintainers.yaml) | 3 | [SECURITY.md](https://github.com/example/.project/blob/main/SECURITY.md) |  | [GOVERNANCE.md](https://raw.githubusercontent.com/example/.project/main/GOVERNANCE.md) |  |\n", markdown)
}

func TestBuildGistReportRowIncludesMaintainersWarning(t *testing.T) {
	t.Parallel()

	row, ok := BuildGistReportRow(model.Project{Name: "Example"}, &DiscoveryResult{
		RepoExists:             true,
		MaintainersFile:        FileDiscovery{Exists: true, BlobURL: "https://github.com/example/.project/blob/main/maintainers.yaml"},
		MaintainersParseStatus: ParseStatusInvalidShape,
		MaintainersParseError:  "maintainers must contain at least one entry",
	})
	require.True(t, ok)

	assert.Equal(t, "maintainers.yaml warning: maintainers must contain at least one entry (https://github.com/example/.project/blob/main/maintainers.yaml)", row.Warning)
}

func TestBuildGistReportRowSkipsMissingRepo(t *testing.T) {
	t.Parallel()

	_, ok := BuildGistReportRow(model.Project{Name: "Example"}, &DiscoveryResult{RepoExists: false})
	assert.False(t, ok)
}
