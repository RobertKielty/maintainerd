package fossa

import "testing"

// TestExtractInvitationEmails covers the org-wide GET /user-invitations
// response shapes we've observed from FOSSA. Regressions here silently
// drop invitations from reconciliation without any error surfacing.
func TestExtractInvitationEmails(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantEmails []string
		wantCount  int
		wantParsed bool
	}{
		{
			name:       "populated invitations",
			body:       `[{"email":"a@example.com"},{"email":"b@example.com"}]`,
			wantEmails: []string{"a@example.com", "b@example.com"},
			wantCount:  2,
			wantParsed: true,
		},
		{
			name:       "empty array",
			body:       `[]`,
			wantEmails: nil,
			wantCount:  0,
			wantParsed: true,
		},
		{
			name:       "whitespace-only body",
			body:       "   ",
			wantEmails: nil,
			wantCount:  0,
			wantParsed: false,
		},
		{
			name:       "malformed json",
			body:       `not json`,
			wantEmails: nil,
			wantCount:  0,
			wantParsed: false,
		},
		{
			name:       "entries with blank email are skipped",
			body:       `[{"email":"a@example.com"},{"email":"  "},{"email":""}]`,
			wantEmails: []string{"a@example.com"},
			wantCount:  3,
			wantParsed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emails, count, parsed := extractInvitationEmails(tt.body)
			if parsed != tt.wantParsed {
				t.Fatalf("parsed = %v, want %v", parsed, tt.wantParsed)
			}
			if count != tt.wantCount {
				t.Fatalf("count = %d, want %d", count, tt.wantCount)
			}
			if len(emails) != len(tt.wantEmails) {
				t.Fatalf("emails = %v, want %v", emails, tt.wantEmails)
			}
			for i, email := range emails {
				if email != tt.wantEmails[i] {
					t.Fatalf("emails[%d] = %q, want %q", i, email, tt.wantEmails[i])
				}
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := normalizeEmail("  Foo@example.com  "); got != "foo@example.com" {
		t.Fatalf("normalizeEmail = %q, want %q", got, "foo@example.com")
	}
}
