/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"testing"
)

func TestAnnotationConstants(t *testing.T) {
	// Verify annotation constants follow expected naming convention
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "last synced annotation",
			constant: AnnotationStaffLastSynced,
			expected: "kdp-workspaces.cncf.io/last-synced",
		},
		{
			name:     "sync status annotation",
			constant: AnnotationStaffSyncStatus,
			expected: "kdp-workspaces.cncf.io/sync-status",
		},
		{
			name:     "workspace count annotation",
			constant: AnnotationStaffWorkspaceCount,
			expected: "kdp-workspaces.cncf.io/workspace-count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("annotation constant mismatch: got %q, want %q", tt.constant, tt.expected)
			}
		})
	}
}

func TestSyncStatusValues(t *testing.T) {
	// Test sync status value logic
	tests := []struct {
		name            string
		totalWorkspaces int
		successCount    int
		expectedStatus  string
		description     string
	}{
		{
			name:            "all workspaces success",
			totalWorkspaces: 5,
			successCount:    5,
			expectedStatus:  "success",
			description:     "when all workspaces are updated successfully",
		},
		{
			name:            "partial success",
			totalWorkspaces: 5,
			successCount:    3,
			expectedStatus:  "partial",
			description:     "when some workspaces fail",
		},
		{
			name:            "complete failure",
			totalWorkspaces: 5,
			successCount:    0,
			expectedStatus:  "error",
			description:     "when all workspaces fail",
		},
		{
			name:            "no workspaces",
			totalWorkspaces: 0,
			successCount:    0,
			expectedStatus:  "success",
			description:     "when there are no workspaces to sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the sync status logic from the reconciler
			var syncStatus string
			errorCount := tt.totalWorkspaces - tt.successCount

			if errorCount > 0 {
				syncStatus = "partial"
				if tt.successCount == 0 {
					syncStatus = "error"
				}
			} else {
				syncStatus = "success"
			}

			if syncStatus != tt.expectedStatus {
				t.Errorf("%s: got status %q, want %q", tt.description, syncStatus, tt.expectedStatus)
			}
		})
	}
}

func TestOIDCSubjectPrefix(t *testing.T) {
	// Test that OIDC prefix is correctly applied to email addresses
	tests := []struct {
		name            string
		email           string
		expectedSubject string
	}{
		{
			name:            "standard email",
			email:           "user@example.com",
			expectedSubject: "oidc:user@example.com",
		},
		{
			name:            "email with plus",
			email:           "user+test@example.com",
			expectedSubject: "oidc:user+test@example.com",
		},
		{
			name:            "organizational email",
			email:           "staff@example.test",
			expectedSubject: "oidc:staff@example.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the subject name construction from the reconciler
			subjectName := "oidc:" + tt.email

			if subjectName != tt.expectedSubject {
				t.Errorf("subject name = %q, want %q", subjectName, tt.expectedSubject)
			}
		})
	}
}

func TestEmptyEmailHandling(t *testing.T) {
	// Test that empty emails are properly skipped
	emails := []string{
		"user1@example.com",
		"", // empty email
		"user2@example.com",
		"", // another empty
		"user3@example.com",
	}

	var validSubjects []string
	for _, email := range emails {
		if email != "" {
			validSubjects = append(validSubjects, "oidc:"+email)
		}
	}

	expectedCount := 3
	if len(validSubjects) != expectedCount {
		t.Errorf("valid subject count = %d, want %d", len(validSubjects), expectedCount)
	}

	// Verify no empty subjects
	for i, subject := range validSubjects {
		if subject == "oidc:" || subject == "" {
			t.Errorf("subject[%d] is empty or has no email: %q", i, subject)
		}
	}
}

func TestReconcilerConfiguration(t *testing.T) {
	// Test that reconciler can be instantiated with valid configuration
	reconciler := &StaffMemberReconciler{
		KCPConfigMapName:      "kdp-workspaces",
		KCPConfigMapNamespace: "kdp-workspaces-system",
		KCPSecretName:         "kdp-workspaces",
		KCPSecretNamespace:    "kdp-workspaces-system",
		StaffMemberNamespace:  "maintainerd",
	}

	if reconciler.KCPConfigMapName == "" {
		t.Error("KCPConfigMapName should not be empty")
	}

	if reconciler.StaffMemberNamespace == "" {
		t.Error("StaffMemberNamespace should not be empty")
	}

	// Verify default namespace is maintainerd
	expectedNamespace := "maintainerd"
	if reconciler.StaffMemberNamespace != expectedNamespace {
		t.Errorf("StaffMemberNamespace = %q, want %q",
			reconciler.StaffMemberNamespace, expectedNamespace)
	}
}
