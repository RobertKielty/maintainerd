package main

import (
	"maintainerd/geo"
	"maintainerd/model"
	"testing"
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
