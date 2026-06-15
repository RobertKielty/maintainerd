package main

import (
	"encoding/json"
	"testing"
	"time"

	"maintainerd/dotproject"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequiredLFXClientRequiresToken(t *testing.T) {
	t.Setenv("LFX_AUTH_TOKEN", "")
	t.Setenv("LFX_ACL", "")

	client, err := buildRequiredLFXClient()
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "https://app.lfx.dev/settings")
}

func TestBuildRequiredLFXClientUsesToken(t *testing.T) {
	t.Setenv("LFX_AUTH_TOKEN", "short-lived-token")
	t.Setenv("LFX_ACL", "acl")

	client, err := buildRequiredLFXClient()
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "short-lived-token", client.Token)
	assert.Equal(t, "acl", client.ACL)
}

func TestBuildLFXClientDefaultRequestDelay(t *testing.T) {
	t.Setenv("LFX_REQUEST_DELAY", "")

	client := buildLFXClient("token", "")

	require.NotNil(t, client)
	assert.Equal(t, 250*time.Millisecond, client.MinDelay)
}

func TestBuildLFXClientRequestDelayOverride(t *testing.T) {
	t.Setenv("LFX_REQUEST_DELAY", "750ms")

	client := buildLFXClient("token", "")

	require.NotNil(t, client)
	assert.Equal(t, 750*time.Millisecond, client.MinDelay)
}

func TestFoundationCSVBlobURLUsesPlainView(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		"https://github.com/cncf/foundation/blob/abc123/project-maintainers.csv?plain=1",
		foundationCSVBlobURL("cncf", "foundation", "abc123", "project-maintainers.csv"),
	)
}

func TestBuildAuditEvent(t *testing.T) {
	t.Parallel()

	event := buildAuditEvent(dotproject.SyncSummary{
		Loaded:              25,
		Total:               20,
		Skipped:             5,
		SkippedArchived:     3,
		SkippedExcluded:     2,
		Synced:              18,
		Errored:             2,
		GitHubErrorCount:    2,
		RateLimitErrorCount: 1,
		NotFound:            10,
		RepoOnly:            4,
		Partial:             3,
		Adopted:             1,
		AutoAdd: dotproject.AutoAddSummary{
			Candidates:             2,
			WouldCreateMaintainers: 1,
			WouldLinkMaintainers:   1,
			WouldCreate: []dotproject.AutoAddCandidateSummary{
				{Project: "Kubernetes", GitHub: "alice"},
			},
			WouldLink: []dotproject.AutoAddCandidateSummary{
				{Project: "Prometheus", GitHub: "bob"},
			},
		},
		WarningSummaries: []string{"Project One: maintainers.yaml warning: maintainers must contain at least one entry (https://github.com/org/.project/blob/main/maintainers.yaml)"},
		ErrorSummaries:   []string{"github rate limit exceeded", "boom"},
	}, &postSyncMetrics{
		DBSizeBytes:              14680064,
		DotProjectSyncStateBytes: 172032,
		CachedFiles:              35,
		MaintainersBodyBytes:     17408,
		AvgMaintainersBodyBytes:  497,
		MaxMaintainersBodyBytes:  1024,
		ProjectsTotal:            35,
		ReposFound:               35,
		CachedBodies:             35,
	}, nil, syncConfig{
		CheckFoundationCSV: true,
		AutoAddMaintainers: false,
		Actor:              "staff-tester",
		FoundationOwner:    "cncf",
		FoundationRepo:     "foundation",
		FoundationRef:      "main",
		FoundationPath:     "project-maintainers.csv",
		GistFilename:       "dot-project-repos.csv",
	})

	assert.Equal(t, "DOT_PROJECT_SYNC_RUN", event.Action)
	assert.Contains(t, event.Message, "scanned=20")
	assert.Contains(t, event.Message, "rate_limit_errors=1")

	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(event.Metadata), &metadata))
	assert.Equal(t, float64(25), metadata["loaded"])
	assert.Equal(t, float64(20), metadata["scanned"])
	assert.Equal(t, float64(5), metadata["skipped"])
	assert.Equal(t, float64(2), metadata["github_error_count"])
	assert.Equal(t, float64(1), metadata["rate_limit_error_count"])
	assert.Equal(t, float64(14680064), metadata["db_size_bytes"])
	assert.Equal(t, float64(172032), metadata["dot_project_sync_state_bytes"])
	assert.Equal(t, float64(35), metadata["cached_files"])
	assert.Equal(t, float64(17408), metadata["maintainers_body_bytes"])
	assert.Equal(t, float64(497), metadata["avg_maintainers_body_bytes"])
	assert.Equal(t, float64(1024), metadata["max_maintainers_body_bytes"])
	assert.Equal(t, float64(35), metadata["projects_total"])
	assert.Equal(t, float64(35), metadata["repos_found"])
	assert.Equal(t, float64(35), metadata["cached_bodies"])
	assert.Equal(t, float64(2), metadata["auto_add_candidates"])
	assert.Equal(t, float64(1), metadata["auto_add_would_create"])
	assert.Equal(t, float64(1), metadata["auto_add_would_link"])
	assert.Equal(t, true, metadata["check_foundation_csv"])
	assert.Equal(t, false, metadata["auto_add_maintainers"])
	assert.Equal(t, "staff-tester", metadata["dot_project_sync_actor"])
	assert.Equal(t, "cncf", metadata["foundation_csv_owner"])
	wouldCreate, ok := metadata["auto_add_would_create_handles"].([]any)
	require.True(t, ok)
	assert.Len(t, wouldCreate, 1)
	errorsValue, ok := metadata["errors"].([]any)
	require.True(t, ok)
	assert.Len(t, errorsValue, 2)
	warningsValue, ok := metadata["warnings"].([]any)
	require.True(t, ok)
	assert.Len(t, warningsValue, 1)
}
