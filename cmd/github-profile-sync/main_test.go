package main

import (
	"encoding/json"
	"testing"
	"time"

	"maintainerd/geo"
	"maintainerd/model"
)

func strPtr(s string) *string { return &s }

func TestDiffResolvedLocation(t *testing.T) {
	tests := []struct {
		name       string
		previous   geo.ResolvedLocation
		resolved   geo.ResolvedLocation
		wantFields []string
	}{
		{
			name:       "no change",
			previous:   geo.ResolvedLocation{Location: strPtr("Berlin, Germany"), Country: strPtr("DE"), Timezone: strPtr("Europe/Berlin")},
			resolved:   geo.ResolvedLocation{Location: strPtr("Berlin, Germany"), Country: strPtr("DE"), Timezone: strPtr("Europe/Berlin")},
			wantFields: nil,
		},
		{
			name:       "first-time resolve",
			previous:   geo.ResolvedLocation{},
			resolved:   geo.ResolvedLocation{Location: strPtr("Berlin, Germany"), Country: strPtr("DE"), Timezone: strPtr("Europe/Berlin")},
			wantFields: []string{"country", "location", "timezone"},
		},
		{
			name:       "cleared entirely",
			previous:   geo.ResolvedLocation{Location: strPtr("Berlin, Germany"), Country: strPtr("DE"), Timezone: strPtr("Europe/Berlin")},
			resolved:   geo.ResolvedLocation{},
			wantFields: []string{"country", "location", "timezone"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes := diffResolvedLocation(tc.previous, tc.resolved)
			if len(changes) != len(tc.wantFields) {
				t.Fatalf("diffResolvedLocation() = %v, want fields %v", changes, tc.wantFields)
			}
			for _, field := range tc.wantFields {
				if _, ok := changes[field]; !ok {
					t.Errorf("diffResolvedLocation() missing field %q, got %v", field, changes)
				}
			}
		})
	}
}

func TestDisplayValueUsesMissingPlaceholder(t *testing.T) {
	if got := displayValue("country", nil); got != "COUNTRY_MISSING" {
		t.Errorf("displayValue(country, nil) = %q, want COUNTRY_MISSING", got)
	}
	if got := displayValue("country", strPtr("DE")); got != "DE" {
		t.Errorf("displayValue(country, &DE) = %q, want DE", got)
	}
}

func TestBuildUpdateAuditEvent(t *testing.T) {
	m := model.Maintainer{GitHubAccount: "octocat"}
	m.ID = 42
	changes := map[string]map[string]string{
		"country": {"from": "COUNTRY_MISSING", "to": "DE"},
	}
	event := buildUpdateAuditEvent(m, changes)
	if event.Action != "GITHUB_PROFILE_SYNC_UPDATE" {
		t.Errorf("Action = %q, want GITHUB_PROFILE_SYNC_UPDATE", event.Action)
	}
	if event.MaintainerID == nil || *event.MaintainerID != 42 {
		t.Errorf("MaintainerID = %v, want 42", event.MaintainerID)
	}
}

func TestErrorRateExcludes404s(t *testing.T) {
	tests := []struct {
		name    string
		summary syncSummary
		want    float64
	}{
		{
			name:    "all errors are 404s",
			summary: syncSummary{Attempted: 10, Errored: 8, NotFoundCount: 8},
			want:    0,
		},
		{
			name:    "no attempts",
			summary: syncSummary{},
			want:    0,
		},
		{
			name: "rate-limit errors count toward the ratio",
			summary: syncSummary{
				Attempted: 10, Errored: 8, NotFoundCount: 0,
				GitHubErrorCount: 8, RateLimitErrorCount: 8,
			},
			want: 0.8,
		},
		{
			name: "mixed 404s and credible errors only count the credible ones",
			summary: syncSummary{
				Attempted: 10, Errored: 8, NotFoundCount: 5,
				GitHubErrorCount: 3,
			},
			want: 0.3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errorRate(tc.summary); got != tc.want {
				t.Errorf("errorRate(%+v) = %v, want %v", tc.summary, got, tc.want)
			}
		})
	}
}

func TestErrorRateExceedsThresholdTriggersNonZeroExit(t *testing.T) {
	summary := syncSummary{Attempted: 10, Errored: 8, GitHubErrorCount: 8, RateLimitErrorCount: 8}
	cfg := syncConfig{MaxErrorRate: 0.5}
	errRate := errorRate(summary)
	exceeded := summary.Attempted > 0 && errRate > cfg.MaxErrorRate
	if !exceeded {
		t.Fatalf("expected error rate %.3f to exceed max-error-rate %.3f", errRate, cfg.MaxErrorRate)
	}

	notFoundSummary := syncSummary{Attempted: 10, Errored: 8, NotFoundCount: 8}
	notFoundRate := errorRate(notFoundSummary)
	if notFoundExceeded := notFoundSummary.Attempted > 0 && notFoundRate > cfg.MaxErrorRate; notFoundExceeded {
		t.Fatalf("expected all-404 error rate %.3f not to exceed max-error-rate %.3f", notFoundRate, cfg.MaxErrorRate)
	}
}

func TestBuildStartAuditEvent(t *testing.T) {
	cfg := syncConfig{Pause: 750 * time.Millisecond, MaxErrorRate: 0.5}
	event := buildStartAuditEvent(cfg)
	if event.Action != "GITHUB_PROFILE_SYNC_RUN_STARTED" {
		t.Errorf("Action = %q, want GITHUB_PROFILE_SYNC_RUN_STARTED", event.Action)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(event.Metadata), &metadata); err != nil {
		t.Fatalf("Metadata is not valid JSON: %v", err)
	}
	if got := metadata["pause_ms"]; got != float64(750) {
		t.Errorf("Metadata[pause_ms] = %v, want 750", got)
	}
	if got := metadata["max_error_rate"]; got != 0.5 {
		t.Errorf("Metadata[max_error_rate] = %v, want 0.5", got)
	}
}

func TestBuildFinishAuditEvent(t *testing.T) {
	summary := syncSummary{
		Attempted: 10, Updated: 3, Cleared: 1, Unchanged: 4, Errored: 2,
		GitHubErrorCount: 2, RateLimitErrorCount: 1, NotFoundCount: 0,
	}
	cfg := syncConfig{MaxErrorRate: 0.1}
	errRate := errorRate(summary)
	exceeded := summary.Attempted > 0 && errRate > cfg.MaxErrorRate
	if !exceeded {
		t.Fatalf("expected error rate %.3f to exceed max-error-rate %.3f for this test's fixture", errRate, cfg.MaxErrorRate)
	}

	event := buildFinishAuditEvent(summary, cfg, errRate, exceeded)
	if event.Action != "GITHUB_PROFILE_SYNC_RUN_FINISHED" {
		t.Errorf("Action = %q, want GITHUB_PROFILE_SYNC_RUN_FINISHED", event.Action)
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(event.Metadata), &metadata); err != nil {
		t.Fatalf("Metadata is not valid JSON: %v", err)
	}
	wantInts := map[string]int{
		"attempted":              summary.Attempted,
		"updated":                summary.Updated,
		"cleared":                summary.Cleared,
		"unchanged":              summary.Unchanged,
		"errored":                summary.Errored,
		"github_error_count":     summary.GitHubErrorCount,
		"rate_limit_error_count": summary.RateLimitErrorCount,
		"not_found_count":        summary.NotFoundCount,
	}
	for field, want := range wantInts {
		got, ok := metadata[field].(float64)
		if !ok || int(got) != want {
			t.Errorf("Metadata[%s] = %v, want %d", field, metadata[field], want)
		}
	}
	if got, ok := metadata["error_rate_exceeded"].(bool); !ok || !got {
		t.Errorf("Metadata[error_rate_exceeded] = %v, want true", metadata["error_rate_exceeded"])
	}
}
