package main

import (
	"encoding/json"
	"testing"

	"maintainerd/dotproject"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		ErrorSummaries:      []string{"github rate limit exceeded", "boom"},
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
	errorsValue, ok := metadata["errors"].([]any)
	require.True(t, ok)
	assert.Len(t, errorsValue, 2)
}
